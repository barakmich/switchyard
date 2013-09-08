# Absolute path to this script. /home/user/bin/foo.sh
SCRIPT=$(readlink -f $0)
# Absolute path this script is in. /home/user/bin
SCRIPTPATH=`dirname $SCRIPT`

#export GOROOT=
export PATH="$PATH:/usr/local/go/bin"
export GOPATH=$SCRIPTPATH:$GOPATH
export GOOS="linux"
export GOARCH="amd64"
gocode close
gocode
