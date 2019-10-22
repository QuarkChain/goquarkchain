package deploy

import (
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"path"
	"time"
)

func sftpConnect(user, password, host string, port int) (*sftp.Client, error) {
	var (
		auth         []ssh.AuthMethod
		addr         string
		clientConfig *ssh.ClientConfig
		sshClient    *ssh.Client
		sftpClient   *sftp.Client
		err          error
	)
	// get auth method
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(password))

	clientConfig = &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		Timeout:         30 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //ssh.FixedHostKey(hostKey),
	}

	// connet to ssh
	addr = fmt.Sprintf("%s:%d", host, port)
	if sshClient, err = ssh.Dial("tcp", addr, clientConfig); err != nil {
		return nil, err
	}

	// create sftp client
	if sftpClient, err = sftp.NewClient(sshClient); err != nil {
		return nil, err
	}
	return sftpClient, nil
}

func UploadFile(sftpClient *sftp.Client, localFilePath string, remotePath string) {
	srcFile, err := os.Open(localFilePath)
	Checkerr(err)
	defer srcFile.Close()

	var remoteFileName = path.Base(localFilePath)

	//fmt.Println("remotePat", remotePath, remoteFileName)
	dstFile, err := sftpClient.Create(path.Join(remotePath, remoteFileName))
	Checkerr(err)
	defer dstFile.Close()

	ff, err := ioutil.ReadAll(srcFile)
	Checkerr(err)
	dstFile.Write(ff)
	//fmt.Println(localFilePath + "  copy file to remote server finished!")
}

func UploadDirectory(sftpClient *sftp.Client, localPath string, remotePath string) {
	localFiles, err := ioutil.ReadDir(localPath)
	Checkerr(err)

	for _, backupDir := range localFiles {
		localFilePath := path.Join(localPath, backupDir.Name())
		remoteFilePath := path.Join(remotePath, backupDir.Name())
		if backupDir.IsDir() {
			sftpClient.Mkdir(remoteFilePath)
			UploadDirectory(sftpClient, localFilePath, remoteFilePath)
		} else {
			UploadFile(sftpClient, path.Join(localPath, backupDir.Name()), remotePath)
		}
	}

	//fmt.Println(localPath + "  copy directory to remote server finished!")
}

func GetfileFromRemote(sftpClient *sftp.Client, localDir string, remotePath string) {
	srcFile, err := sftpClient.Open(remotePath)
	Checkerr(err)
	defer srcFile.Close()

	var localFileName = path.Base(remotePath)
	dstFile, err := os.Create(path.Join(localDir, localFileName))
	Checkerr(err)
	defer dstFile.Close()

	if _, err = srcFile.WriteTo(dstFile); err != nil {
		fmt.Println("err", err, remotePath)
		panic("sb")
	}

	//fmt.Println("copy file from remote server finished!!!!!!!!!!!!!")
}
