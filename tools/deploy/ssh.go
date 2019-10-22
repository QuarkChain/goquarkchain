package deploy

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/crypto/ssh"
	"net"
)

type SSHSession struct {
	user, password, host string
	port                 int
}

func NewSSHConnect(user, password, host string, port int) *SSHSession {

	return &SSHSession{user, password, host, port}
}

func (s *SSHSession) RunCmd(cmd string) {
	log.Info("cmd", "host", s.host, "port", s.port, "user", s.user, "port", s.password, "cmd", cmd)
	var stdOut, stdErr bytes.Buffer
	session, err := SSHConnect(s.user, s.password, s.host, s.port)
	Checkerr(err)
	defer session.Close()
	session.Stdout = &stdOut
	session.Stderr = &stdErr
	session.Run(cmd)
	if stdOut.String() != "" {
		//fmt.Println("out", stdOut.String())
	}
	if stdErr.String() != "" {
		//fmt.Println("err", stdErr.String())
	}
}

func (s *SSHSession) RunMiner(branch int64, minerNumber int) {
	for index := 0; index < minerNumber; index++ {
		cmd := "docker exec -itd bjqkc /bin/bash -c  'chmod +x /tmp/QKC/miner && /tmp/QKC/miner -config /tmp/QKC/cluster_config_template.json  -host 127.0.0.1 -port 38391 "
		minerAddress := "0x"
		for addrIndex := 0; addrIndex < 40; addrIndex++ {
			minerAddress += string(byte(index + '0'))
		}

		if branch < 0 {
			// master
		} else {
			shardFlag := fmt.Sprintf(" -shards %v ", branch)
			cmd += shardFlag
		}
		cmd += " -coinbase "
		cmd += minerAddress
		cmd += "'"
		s.RunCmd(cmd)
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
		User: user,
		Auth: auth,
		// Timeout:             30 * time.Second,
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
	Checkerr(err)
	UploadFile(sftp, localFile, remoteFile)
}

func (s *SSHSession) GetFile(localDir string, remoteFile string) {
	sftp, err := sftpConnect(s.user, s.password, s.host, s.port)
	Checkerr(err)
	GetfileFromRemote(sftp, localDir, remoteFile)
}
