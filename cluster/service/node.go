// Modified from go-ethereum under GNU Lesser General Public License
package service

import (
	"fmt"
	qkcrpc "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/prometheus/prometheus/util/flock"
	"google.golang.org/grpc"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
)

// Node is a container on which services can be registered.
type Node struct {
	eventmux *event.TypeMux // Event multiplexer used between the services of a stack
	config   *Config

	instanceDirLock flock.Releaser // prevents concurrent use of instance directory

	serverConfig p2p.Config
	server       *p2p.Server // Currently running P2P networking layer

	serviceFuncs []ServiceConstructor     // Service constructors (in dependency order)
	services     map[reflect.Type]Service // Currently running services

	rpcAPIs       []rpc.API   // List of APIs currently provided by the node
	inprocHandler *rpc.Server // In-process RPC request handler to process the API requests

	ipcListener net.Listener // IPC RPC listener socket to serve API requests
	ipcHandler  *rpc.Server  // IPC RPC request handler to process the API requests

	httpWhitelist []string     // public HTTP RPC modules to allow through this endpoint
	httpListener  net.Listener // public HTTP RPC listener socket to server API requests
	httpHandler   *rpc.Server  // public HTTP RPC request handler to process the API requests

	httpPrivListener net.Listener // private HTTP RPC listener socket to server API requests
	httpPrivHandler  *rpc.Server  // private HTTP RPC request handler to process the API requests

	isMaster     bool         // node module, true master type full functions start, false slave type just start part functions.
	grpcListener net.Listener // GRPC listener socket to server API requests
	grpcHandler  *grpc.Server // GRPC request handler to process the API requests

	wsListener net.Listener // Websocket RPC listener socket to server API requests
	wsHandler  *rpc.Server  // Websocket RPC request handler to process the API requests

	stop chan struct{} // Channel to wait for termination notifications
	lock sync.RWMutex

	sigc chan os.Signal
	log  log.Logger
}

// New creates a new P2P node, ready for protocol registration.
func New(conf *Config) (*Node, error) {
	// Copy config and resolve the datadir so future changes to the current
	// working directory don't affect the node.
	confCopy := *conf
	conf = &confCopy
	if conf.DataDir != "" {
		absdatadir, err := filepath.Abs(conf.DataDir)
		if err != nil {
			return nil, err
		}
		conf.DataDir = absdatadir
	}
	if conf.Logger == nil {
		conf.Logger = log.New()
	}
	// Note: any interaction with Config that would create/touch files
	// in the data directory or instance directory is delayed until Start.
	node := &Node{
		config:       conf,
		serviceFuncs: []ServiceConstructor{},
		eventmux:     new(event.TypeMux),
		isMaster:     true,
		sigc:         make(chan os.Signal, 1),
		log:          conf.Logger,
	}
	return node, nil
}

func (n *Node) GetSigc() chan os.Signal {
	return n.sigc
}

func (n *Node) SetIsMaster(isMaster bool) {
	n.isMaster = isMaster
}

func (n *Node) IsMaster() bool {
	return n.isMaster
}

// Register injects a new service into the node's stack. The service created by
// the passed constructor must be unique in its type with regard to sibling ones.
func (n *Node) Register(constructor ServiceConstructor) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.server != nil {
		return ErrNodeRunning
	}
	n.serviceFuncs = append(n.serviceFuncs, constructor)
	return nil
}

// Start create a live P2P node and starts running it.
func (n *Node) Start() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	// Short circuit if the node's already running
	if n.server != nil {
		return ErrNodeRunning
	}
	if err := n.openDataDir(); err != nil {
		return err
	}

	// Initialize the p2p server. This creates the node key and
	// discovery databases.
	n.serverConfig = n.config.P2P
	n.serverConfig.PrivateKey = n.config.NodeKey()
	n.serverConfig.Name = n.config.NodeName()
	n.serverConfig.Logger = n.log
	if n.serverConfig.StaticNodes == nil {
		n.serverConfig.StaticNodes = n.config.StaticNodes()
	}
	if n.serverConfig.TrustedNodes == nil {
		n.serverConfig.TrustedNodes = n.config.TrustedNodes()
	}
	if n.serverConfig.NodeDatabase == "" {
		n.serverConfig.NodeDatabase = n.config.NodeDB()
	}
	running := &p2p.Server{Config: n.serverConfig}

	// Otherwise copy and specialize the P2P configuration
	services := make(map[reflect.Type]Service)
	for _, constructor := range n.serviceFuncs {
		// Create a new context for the particular service
		ctx := &ServiceContext{
			config:   n.config,
			services: make(map[reflect.Type]Service),
			Shutdown: n.sigc,
			EventMux: n.eventmux,
		}
		for kind, s := range services { // copy needed for threaded access
			ctx.services[kind] = s
		}
		// Construct and save the service
		service, err := constructor(ctx)
		if err != nil {
			return err
		}
		kind := reflect.TypeOf(service)
		if _, exists := services[kind]; exists {
			return &DuplicateServiceError{Kind: kind}
		}
		services[kind] = service
	}
	// Gather the protocols and start the freshly assembled P2P server
	for _, service := range services {
		running.Protocols = append(running.Protocols, service.Protocols()...)
	}
	if n.IsMaster() {
		if err := running.Start(); err != nil {
			return convertFileLockError(err)
		}
		n.log.Info("Starting peer-to-peer node", "instance", n.serverConfig.Name)
	}
	// Lastly start the configured RPC interfaces
	if err := n.startRPC(services); err != nil {
		running.Stop()
		return err
	}
	// Start each of the services
	var started []reflect.Type
	for kind, service := range services {
		// Start the next service, stopping all previous upon failure
		if err := service.Init(running); err != nil {
			for _, kind := range started {
				services[kind].Stop()
			}
			running.Stop()
			n.stopRPC()
			return err
		}
		// Mark the service started for potential cleanup
		started = append(started, kind)
	}
	// Finish initializing the startup
	n.services = services
	n.server = running
	n.stop = make(chan struct{})
	return nil
}

func (n *Node) openDataDir() error {
	if n.config.DataDir == "" {
		return nil // ephemeral
	}

	instdir := filepath.Join(n.config.DataDir, n.config.name())
	if err := os.MkdirAll(instdir, 0700); err != nil {
		return err
	}
	// Lock the instance directory to prevent concurrent use by another instance as well as
	// accidental use of the instance directory as a database.
	release, _, err := flock.New(filepath.Join(instdir, "LOCK"))
	if err != nil {
		return convertFileLockError(err)
	}
	n.instanceDirLock = release
	return nil
}

// startRPC is a helper method to start all the various RPC endpoint during node
// startup. It's not meant to be called at any time afterwards as it makes certain
// assumptions about the state of the node.
func (n *Node) startRPC(services map[reflect.Type]Service) error {
	// Gather all the possible APIs to surface
	apis := make([]rpc.API, 0, len(services))
	for _, srv := range services {
		apis = append(apis, srv.APIs()...)
	}
	if err := n.startGRPC(apis, n.config.GRPCModules); err != nil {
		n.stopRPC()
		return err
	}

	// start ws service
	if err := n.startWS(apis, n.config.WSModules, n.config.WSOrigins); err != nil {
		n.stopWS()
		return err
	}

	if n.IsMaster() {
		if err := n.startIPC(apis); err != nil {
			n.stopRPC()
			return err
		}
		if err := n.startHTTP(apis, n.config.HTTPModules, n.config.HTTPTimeouts); err != nil {
			n.stopRPC()
			return err
		}
		if err := n.startPrivHTTP(apis, n.config.HTTPModules, n.config.HTTPTimeouts); err != nil {
			n.stopRPC()
			return err
		}
	}
	// All API endpoints started successfully
	n.rpcAPIs = apis
	return nil
}

func (n *Node) stopRPC() {
	n.stopGRPC()
	n.stopWS()
	if n.IsMaster() {
		n.stopIPC()
		n.stopHTTP()
		n.stopPrivHTTP()
	}
}

func (n *Node) apiFilter(nodeApis []rpc.API, isPublic bool, modules []string) []rpc.API {
	apis := make([]rpc.API, 0)
	for _, module := range modules {
		for _, api := range nodeApis {
			if api.Namespace == module && api.Public == isPublic {
				apis = append(apis, api)
			}
		}
	}
	return apis
}

// startIPC initializes and starts the IPC RPC endpoint.
func (n *Node) startIPC(apis []rpc.API) error {
	if n.config.IPCPath == "" {
		return nil // IPC disabled.
	}
	listener, handler, err := rpc.StartIPCEndpoint(n.config.IPCPath, apis)
	if err != nil {
		return err
	}
	n.ipcListener = listener
	n.ipcHandler = handler
	n.log.Info("IPC endpoint opened", "url", n.config.IPCPath)
	return nil
}

// stopIPC terminates the IPC RPC endpoint.
func (n *Node) stopIPC() {
	if n.ipcListener != nil {
		n.ipcListener.Close()
		n.ipcListener = nil

		n.log.Info("IPC endpoint closed", "url", n.config.IPCPath)
	}
	if n.ipcHandler != nil {
		n.ipcHandler.Stop()
		n.ipcHandler = nil
	}
}

func (n *Node) startGRPC(apis []rpc.API, modules []string) error {
	if n.config.GRPCEndpoint == "" {
		return nil // grpc disapbled
	}

	apis = n.apiFilter(apis, false, modules)
	listener, handler, err := qkcrpc.StartGRPCServer(n.config.GRPCEndpoint, apis)
	if err != nil {
		return err
	}
	n.grpcListener = listener
	n.grpcHandler = handler
	n.log.Info("grpc endpoint opened", "url", n.config.GRPCEndpoint)
	return nil
}

func (n *Node) stopGRPC() {
	if n.grpcListener != nil {
		n.grpcListener.Close()
		n.grpcListener = nil

		n.log.Info("grpc endpoint closed", "url", n.config.GRPCEndpoint)
	}
	if n.grpcHandler != nil {
		n.grpcHandler.Stop()
		n.grpcHandler = nil
	}
}

// startWS initializes and starts the websocket RPC endpoint.
func (n *Node) startWS(apis []rpc.API, modules []string, wsOrigins []string) error {
	// Short circuit if the WS endpoint isn't being exposed
	if n.config.WSEndpoint == "" {
		return nil
	}
	listener, handler, err := rpc.StartWSEndpoint(n.config.WSEndpoint, apis, modules, wsOrigins, false)
	if err != nil {
		return err
	}
	n.log.Info("WebSocket endpoint opened", "url", fmt.Sprintf("ws://%s", listener.Addr()))
	// All listeners booted successfully
	n.wsListener = listener
	n.wsHandler = handler

	return nil
}

// stopWS terminates the websocket RPC endpoint.
func (n *Node) stopWS() {
	if n.wsListener != nil {
		n.wsListener.Close()
		n.wsListener = nil

		n.log.Info("WebSocket endpoint closed", "url", fmt.Sprintf("ws://%s", n.config.WSEndpoint))
	}
	if n.wsHandler != nil {
		n.wsHandler.Stop()
		n.wsHandler = nil
	}
}

// startHTTP initializes and starts the HTTP RPC endpoint.
func (n *Node) startHTTP(apis []rpc.API, modules []string, timeouts rpc.HTTPTimeouts) error {
	// Short circuit if the HTTP endpoint isn't being exposed
	if n.config.HTTPEndpoint == "" {
		return nil
	}
	var (
		publicApis = n.apiFilter(apis, true, modules)
		eptParams  []string
	)
	listener, handler, err := rpc.StartHTTPEndpoint(n.config.HTTPEndpoint, publicApis, modules, eptParams, eptParams, timeouts)
	if err != nil {
		return err
	}
	n.log.Info("public HTTP endpoint opened", "url", fmt.Sprintf("http://%s", n.config.HTTPEndpoint))
	// All listeners booted successfully
	n.httpListener = listener
	n.httpHandler = handler

	return nil
}

// stopHTTP terminates the HTTP RPC endpoint.
func (n *Node) stopHTTP() {
	if n.httpListener != nil {
		n.httpListener.Close()
		n.httpListener = nil

		n.log.Info("public HTTP endpoint closed", "url", fmt.Sprintf("http://%s", n.config.HTTPEndpoint))
	}
	if n.httpHandler != nil {
		n.httpHandler.Stop()
		n.httpHandler = nil
	}
}

func (n *Node) startPrivHTTP(apis []rpc.API, modules []string, timeouts rpc.HTTPTimeouts) error {
	if n.config.HTTPPrivEndpoint == "" {
		return nil
	}
	var (
		privateApis = n.apiFilter(apis, false, modules)
		eptParams   []string
	)

	listener, handler, err := rpc.StartHTTPEndpoint(n.config.HTTPPrivEndpoint, privateApis, modules, eptParams, eptParams, timeouts)
	if err != nil {
		return err
	}
	n.httpPrivListener = listener
	n.httpPrivHandler = handler
	n.log.Info("private HTTP endpoint opened", "url", fmt.Sprintf("http://%s", n.config.HTTPPrivEndpoint))
	return nil
}

func (n *Node) stopPrivHTTP() {
	if n.httpPrivListener != nil {
		n.httpPrivListener.Close()
		n.httpPrivListener = nil

		n.log.Info("private HTTP endpoint closed", "url", fmt.Sprintf("http://%s", n.config.HTTPPrivEndpoint))
	}
}

// Stop terminates a running node along with all it's services. In the node was
// not started, an error is returned.
func (n *Node) Stop() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	// Short circuit if the node's not running
	if n.server == nil {
		return ErrNodeStopped
	}

	// Terminate the API, services and the p2p server.
	n.stopRPC()
	n.rpcAPIs = nil
	failure := &StopError{
		Services: make(map[reflect.Type]error),
	}
	for kind, service := range n.services {
		if err := service.Stop(); err != nil {
			failure.Services[kind] = err
		}
	}
	n.server.Stop()
	n.services = nil
	n.server = nil

	// Release instance directory lock.
	if n.instanceDirLock != nil {
		if err := n.instanceDirLock.Release(); err != nil {
			n.log.Error("Can't release datadir lock", "err", err)
		}
		n.instanceDirLock = nil
	}

	// unblock n.Wait
	close(n.stop)

	if len(failure.Services) > 0 {
		return failure
	}
	return nil
}

// Wait blocks the thread until the node is stopped. If the node is not running
// at the time of invocation, the method immediately returns.
func (n *Node) Wait() {
	n.lock.RLock()
	if n.server == nil {
		n.lock.RUnlock()
		return
	}
	stop := n.stop
	n.lock.RUnlock()

	<-stop
}

// Restart terminates a running node and boots up a new one in its place. If the
// node isn't running, an error is returned.
func (n *Node) Restart() error {
	if err := n.Stop(); err != nil {
		return err
	}
	if err := n.Start(); err != nil {
		return err
	}
	return nil
}

// Attach creates an RPC client attached to an in-process API handler.
func (n *Node) Attach() (*rpc.Client, error) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	if n.server == nil {
		return nil, ErrNodeStopped
	}
	return rpc.DialInProc(n.inprocHandler), nil
}

// RPCHandler returns the in-process RPC request handler.
func (n *Node) RPCHandler() (*rpc.Server, error) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	if n.inprocHandler == nil {
		return nil, ErrNodeStopped
	}
	return n.inprocHandler, nil
}

// Server retrieves the currently running P2P network layer. This method is meant
// only to inspect fields of the currently running server, life cycle management
// should be left to this Node entity.
func (n *Node) Server() *p2p.Server {
	n.lock.RLock()
	defer n.lock.RUnlock()

	return n.server
}

// Service retrieves a currently running service registered of a specific type.
func (n *Node) Service(service interface{}) error {
	n.lock.RLock()
	defer n.lock.RUnlock()

	// Short circuit if the node's not running
	if n.server == nil {
		return ErrNodeStopped
	}
	// Otherwise try to find the service to return
	element := reflect.ValueOf(service).Elem()
	if running, ok := n.services[element.Type()]; ok {
		element.Set(reflect.ValueOf(running))
		return nil
	}
	return ErrServiceUnknown
}

// DataDir retrieves the current datadir used by the protocol stack.
// Deprecated: No files should be stored in this directory, use InstanceDir instead.
func (n *Node) DataDir() string {
	return n.config.DataDir
}

// InstanceDir retrieves the instance directory used by the protocol stack.
func (n *Node) InstanceDir() string {
	return n.config.instanceDir()
}

// IPCEndpoint retrieves the current IPC endpoint used by the protocol stack.
func (n *Node) IPCEndpoint() string {
	return n.config.IPCPath
}

// HTTPEndpoint retrieves the current HTTP endpoint used by the protocol stack.
func (n *Node) HTTPEndpoint() string {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.httpListener != nil {
		return n.httpListener.Addr().String()
	}
	return n.config.HTTPEndpoint
}

// EventMux retrieves the event multiplexer used by all the network services in
// the current protocol stack.
func (n *Node) EventMux() *event.TypeMux {
	return n.eventmux
}

// ResolvePath returns the absolute path of a resource in the instance directory.
func (n *Node) ResolvePath(x string) string {
	return n.config.ResolvePath(x)
}

// apis returns the collection of RPC descriptors this node offers.
func (n *Node) apis() []rpc.API {
	return []rpc.API{}
}
