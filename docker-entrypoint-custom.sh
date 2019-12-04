#!/bin/sh

NODES_ROOT=/fusion-node
DATA_DIR=$NODES_ROOT/data
KEYSTORE_DIR=$DATA_DIR/keystore

mainnet="false"
testnet="false"
devnet="false"

nodename=""
statserver=""
unlock="false"
autobt="false"
mining="false"
enable_rpc="false"
rpcaddr=""
rpccorsdomain=""
enable_ws="false"
wsaddr=""
wsorigins=""

display_usage() {
    echo -e "Options:\n"
    echo -e "-h|--help                Print this help information"
    echo -e "-n|--nodename     value  Reporting nodename to ethstats server"
    echo -e "-s|--statserver   value  Specify ethstats server"
    echo -e "-u|--unlock              Unlock account"
    echo -e "-a|--autobt              Auto buy tickets"
    echo -e "-m|--mining              Enable mining"
    echo -e "--testnet                Run testnet"
    echo -e "--devnet                 Run devnet"
    echo -e "--rpc                    Enable the HTTP-RPC server"
    echo -e "--rpcaddr         value  HTTP-RPC server listening interface (default: \"localhost\")"
    echo -e "--rpccorsdomain   value  Comma separated list of domains from which to accept cross origin requests (browser enforced)"
    echo -e "--ws                     Enable the WS-RPC server"
    echo -e "--wsaddr          value  WS-RPC server listening interface (default: \"localhost\")"
    echo -e "--wsorigins       value  Origins from which to accept websockets requests"

    echo -e "\nExample:\n"
    echo 'docker run --name fusion -it --restart unless-stopped -p 9100:9000/tcp -p 9101:9001/tcp -p 41408:40408/tcp -p 41408:40408/udp -v /opt/run/docker/mainnet:/fusion-node jowenshaw/efsn:latest --nodename xxx --unlock --mining --autobt'
    echo ""
    echo 'docker run --name fusion-testnet -it --restart unless-stopped -p 9200:9000/tcp -p 9201:9001/tcp -p 42408:40408/tcp -p 42408:40408/udp -v /opt/run/docker/testnet:/fusion-node jowenshaw/efsn:latest --testnet --rpc --rpcaddr 0.0.0.0 --rpccorsdomain \\* --ws --wsaddr 0.0.0.0 --wsorigins \\* --unlock --mining --autobt'
    echo -e "\nPrerequisite:\n"
    echo "If you want to unlock an account in container startup, please put your keystore file 'UTC.json' and password file 'password.txt' in the directory which is bind mount to $NODES_ROOT (that is '/opt/run/docker/mainnet' in the first example) and note that you have read/write permission to this directory. When starting container, UTC.json will move to $KEYSTORE_DIR, and password.txt will move to $DATA_DIR"
}

while [ "$1" != "" ]; do
    case $1 in
        -n | --nodename )       shift
                                nodename=$1
                                ;;
        -s | --statserver)      shift
                                statserver=$1
                                ;;
        -u | --unlock )         unlock="true"
                                ;;
        -a | --autobt )         autobt="true"
                                ;;
        -m | --mining )         mining="true"
                                ;;
        --testnet)              testnet="true"
                                ;;
        --devnet)               devnet="true"
                                ;;
        --rpc)                  enable_rpc="true"
                                ;;
        --rpcaddr )             shift
                                rpcaddr=$1
                                ;;
        --rpccorsdomain )       shift
                                rpccorsdomain=$1
                                ;;
        --ws)                   enable_ws="true"
                                ;;
        --wsaddr )              shift
                                wsaddr=$1
                                ;;
        --wsorigins )           shift
                                wsorigins=$1
                                ;;
        * )                     display_usage
                                exit 1
    esac
    shift
done

# create keystore folder if does not exit
[ ! -d "$KEYSTORE_DIR" ] && mkdir -p $KEYSTORE_DIR

# format command option
if [ "$testnet" = "true" ]; then
    cmd_options="--testnet"
elif [ "$devnet" = "true" ]; then
    cmd_options="--devnet"
else
    mainnet="true"
    [ -z "$statserver" ] && statserver="fsnMainnet@node.fusionnetwork.io"
fi

cmd_options="$cmd_options --datadir $DATA_DIR"

if [ -n "$nodename" ] && [ -n "$statserver" ]; then
    ethstats=" --ethstats $nodename:$statserver"
    cmd_options=$cmd_options$ethstats
fi

if [ "$unlock" = "true" ]; then
    # store keystore file
    if [ -f "$NODES_ROOT/UTC.json" ]; then
        mv $NODES_ROOT/UTC.json $KEYSTORE_DIR/
        chown root:root $KEYSTORE_DIR/UTC.json
        chmod 600 $KEYSTORE_DIR/UTC.json
    fi
    # store password file
    if [ -f "$NODES_ROOT/password.txt" ]; then
        mv $NODES_ROOT/password.txt $DATA_DIR/
        chown root:root $DATA_DIR/password.txt
        chmod 600 $DATA_DIR/password.txt
    fi
    # get account address
    address=0x$(cat "$KEYSTORE_DIR/UTC.json" 2>/dev/null | grep -Eo "\"address\"\s*:\s*\"[0-9a-fA-F]{40}\"" | awk -F\" '{print $4}')

    unlock=" --unlock $address --password $DATA_DIR/password.txt"
    cmd_options=$cmd_options$unlock
fi

if [ "$autobt" = "true" ]; then
    autobt=" --autobt"
    cmd_options=$cmd_options$autobt
fi

if [ "$mining" = "true" ]; then
    mining=" --mine"
    cmd_options=$cmd_options$mining
fi

if [ "$enable_rpc" = "true" ]; then
    rpc_opts=' --rpc --rpcport 9000 --rpcapi="eth,net,fsn,fsntx"'
    [ -n "$rpcaddr" ] && rpc_opts="$rpc_opts --rpcaddr $rpcaddr"
    [ -n "$rpccorsdomain" ] && rpc_opts="$rpc_opts --rpccorsdomain $rpccorsdomain"
    cmd_options=$cmd_options$rpc_opts
fi

if [ "$enable_ws" = "true" ]; then
    ws_opts=' --ws --wsport 9001 --wsapi="eth,net,fsn,fsntx"'
    [ -n "$wsaddr" ] && ws_opts="$ws_opts --wsaddr $wsaddr"
    [ -n "$wsorigins" ] && ws_opts="$ws_opts --wsorigins $wsorigins"
    cmd_options=$cmd_options$ws_opts
fi

echo "efsn flags: $cmd_options"
eval "efsn $cmd_options"

#/* vim: set ts=4 sts=4 sw=4 et : */
