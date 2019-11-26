OS=`uname -s`
if [ ${OS} == "Darwin"  ];then
    echo "it is max os"
	/usr/bin/ruby -e "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)"
    brew cask install docker
    docker -v
elif [ ${OS} == "Linux"  ];then
    source /etc/os-release
    case $ID in
        debian|ubuntu|devuan)
	    echo "it is ubuntu "
            sudo apt-get update && apt-get install docker.io -y
			docker -v
            ;;
        centos|fedora|rhel)
	    echo "it is centos"
            yumdnf="yum"
            if test "$(echo "$VERSION_ID >= 22" | bc)" -ne 0;
            then
                yumdnf="dnf"
            fi
            sudo $yumdnf update
			sudo $yumdnf -y install docker
            sudo systemctl start docker
			docker -v
            ;;
        *)
            exit 1
            ;;
    esac
else
    echo "Other OS: ${OS}"
fi