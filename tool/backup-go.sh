sshpass -p $PriKey scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null u446960@u446960.your-storagebox.de:public-data/VERSION-GO VERSION-GO

filename=`cat VERSION-GO`

if [ $filename = "Archive-Go-A.tar.gz" ]; then
  filename="Archive-Go-B.tar.gz"
else
  filename="Archive-Go-A.tar.gz"
fi
echo $filename


rm $filename
curl https://goqkcmainnet.s3.amazonaws.com/data/`curl https://goqkcmainnet.s3.amazonaws.com/data/LATEST`.tar.gz --output $filename

sshpass -p $PriKey scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null $filename u446960@u446960.your-storagebox.de:public-data/$filename

echo $filename > VERSION-GO

sshpass -p $PriKey scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null VERSION-GO u446960@u446960.your-storagebox.de:public-data/VERSION-GO

rm $filename