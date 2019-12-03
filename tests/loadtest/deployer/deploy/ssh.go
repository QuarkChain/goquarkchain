package deploy

import (
	"bytes"
	"fmt"
	"net"
	"strings"
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

func (s *SSHSession) installDocker() {
	osInfo := s.RunCmdAndGetOutPut("cat /etc/os-release | grep ID=")
	if strings.Contains(osInfo, "ubuntu") {
		log.Info("ubuntu:ready to install docker", "host", s.host)
		s.RunCmd("apt-get update && apt-get install docker.io")
	} else if strings.Contains(osInfo, "centos") {
		log.Info("centos:ready to install docker", "host", s.host)
		s.RunCmd("yum remove docker  docker-client  docker-client-latest  docker-common  docker-latest  docker-latest-logrotate  docker-logrotate  docker-selinux  docker-engine-selinux  docker-engine docker-ce -y")
		s.RunCmd("rm -rf /var/lib/docker")
		s.RunCmd("yum install -y yum-utils device-mapper-persistent-data lvm2")
		s.RunCmd("yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo")
		s.RunCmd("yum-config-manager --add-repo http://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo")
		s.RunCmd("yum install docker-ce-18.06.1.ce -y")
		s.RunCmd("systemctl start docker")
	} else {
		log.Error("Other os", "info", osInfo)
	}
}

func (s *SSHSession) pullImages(hostWithFullImages *SSHSession, imagesName string) {
	fileStatus := hostWithFullImages.RunCmdAndGetOutPut("ls qkc.img")
	if !strings.Contains(fileStatus, "qkc.img") {
		saveCmd := "docker save > qkc.img " + imagesName
		hostWithFullImages.RunCmd(saveCmd)
	}
	hostWithFullImages.RunCmd("")

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
	return ""
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
