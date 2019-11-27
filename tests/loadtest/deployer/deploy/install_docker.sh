OS=`uname -s`
if [ ${OS} == "Darwin"  ];then
    echo "mac"
elif [ ${OS} == "Linux"  ];then
    source /etc/os-release
    case $ID in
        debian|ubuntu|devuan)
	        echo "ubuntu"
	        apt-get update
	        apt-get install docker.io -y
            ;;
        centos|fedora|rhel)
	    echo "centos"
            yumdnf="yum"
            if test "$(echo "$VERSION_ID >= 22" | bc)" -ne 0;
            then
                yumdnf="dnf"
            fi
            $yumdnf update
	        $yumdnf -y install docker
	        systemctl start docker
            ;;
        *)
            exit 1
            ;;
    esac
else
    echo "Other OS: ${OS}"
fi
