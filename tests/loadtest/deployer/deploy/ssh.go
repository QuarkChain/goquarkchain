package deploy

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/crypto/ssh"
)

type SSHSession struct {
	user, password, host string
	port                 int
}

func NewSSHConnect(user, password, host string, port int) *SSHSession {
	return &SSHSession{user, password, host, port}
}

func (s *SSHSession) RunCmdIgnoreErr(cmd string) {
	var stdOut, stdErr bytes.Buffer
	session, err := SSHConnect(s.user, s.password, s.host, s.port)
	CheckErr(err)
	defer func() {
		session.Close()
	}()
	session.Stdout = &stdOut
	session.Stderr = &stdErr
	err = session.Run(cmd)
	if err != nil {
		//log.Error("run cmd err", "host", s.host, "cmd", cmd, "err", err)
	}
	log.Debug("run cmd", "host", s.host, "cmd", cmd)
}
func (s *SSHSession) RunCmd(cmd string) {
	var stdOut, stdErr bytes.Buffer
	session, err := SSHConnect(s.user, s.password, s.host, s.port)
	CheckErr(err)
	defer func() {
		session.Close()
	}()
	session.Stdout = &stdOut
	session.Stderr = &stdErr
	err = session.Run(cmd)
	if err != nil {
		log.Error("run cmd err", "host", s.host, "cmd", cmd, "err", err)
		return
	} else {
		log.Debug("run cmd", "host", s.host, "cmd", cmd)
	}
	if stdOut.String() != "" {
		//TODO need print?
	}
	if stdErr.String() != "" {
		log.Error("run cmd err", "cmd", cmd, "err", stdErr.String())
	}
}

func (s *SSHSession) RunCmdAndGetOutPut(cmd string) string {
	var stdOut, stdErr bytes.Buffer
	session, err := SSHConnect(s.user, s.password, s.host, s.port)
	CheckErr(err)
	defer func() {
		session.Close()
	}()
	session.Stdout = &stdOut
	session.Stderr = &stdErr
	err = session.Run(cmd)
	if err != nil {
		log.Error("run cmd err", "host", s.host, "cmd", cmd, "err", err)
		return err.Error()
	} else {
		log.Debug("run cmd", "host", s.host, "cmd", cmd)
	}
	if stdOut.String() != "" {
		//TODO need print?
		return stdOut.String()
	}
	if stdErr.String() != "" {
		log.Error("run cmd err", "cmd", cmd, "err", stdErr.String())
	}
}

func SSHConnect(user, password, host string, port int) (*ssh.Session, error) {
	var (
		auth         []ssh.AuthMethod
		addr         string
		clientConfig *ssh.ClientConfig
		client       *ssh.Client
		session      *ssh.Session
		err          error
	)
	// get auth method
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(password))

	hostKeyCallbk := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return nil
	}

	clientConfig = &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		Timeout:         30 * time.Second,
		HostKeyCallback: hostKeyCallbk,
	}

	// connet to ssh
	addr = fmt.Sprintf("%s:%d", host, port)
	if client, err = ssh.Dial("tcp", addr, clientConfig); err != nil {
		return nil, err
	}

	// create session
	if session, err = client.NewSession(); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *SSHSession) SendFile(localFile, remoteFile string) {
	sftp, err := sftpConnect(s.user, s.password, s.host, s.port)
	CheckErr(err)
	UploadFile(sftp, localFile, remoteFile)
}

func (s *SSHSession) GetFile(localDir string, remoteFile string) {
	sftp, err := sftpConnect(s.user, s.password, s.host, s.port)
	CheckErr(err)
	GetfileFromRemote(sftp, localDir, remoteFile)
}
