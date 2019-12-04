package deploy

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
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
	CheckErr(err)
	defer srcFile.Close()

	var remoteFileName = path.Base(localFilePath)

	dstFile, err := sftpClient.Create(path.Join(remotePath, remoteFileName))

	CheckErr(err)
	defer dstFile.Close()

	ff, err := ioutil.ReadAll(srcFile)
	CheckErr(err)
	dstFile.Write(ff)
}

func GetfileFromRemote(sftpClient *sftp.Client, localDir string, remotePath string) {
	srcFile, err := sftpClient.Open(remotePath)
	srcFile.Chmod(os.ModePerm)
	CheckErr(err)
	defer srcFile.Close()

	var localFileName = path.Base(remotePath)
	dstFile, err := os.Create(path.Join(localDir, localFileName))
	CheckErr(err)
	defer dstFile.Close()

	if _, err = srcFile.WriteTo(dstFile); err != nil {
		panic(err)
	}
}
