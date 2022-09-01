# KafkaStatus

This tool is aimed at gathering information from some Kafka clusters. Some advantages over a simple kafka command is:
*  the possibility to target several clusters at once (using comma as names separator e.g. cluster1,cluster2,cluster3)
* the same applies for topic names (e.g. topic1,topic2)
* to run all sub-commands as concurrent tasks, speeding the process drastically
* to get some health check for all cluster of a git branch

### Available Commands:

```
  acl         [ERDING] Display acls of all or subset topics of a cluster
  config      [ERDING] Display the config (static and dynamic) for the given cluster
  group       [ERDING] Check group info of a cluster
  health      [ERDING] Check health info of a cluster
  help        Help about any command
  info        [ERDING] Display some stats of the given cluster(s)
  inventory   [ERDING] Build a ansible-like inventory based on a git branch
  kgroup      [PaaS] Display groups info inside a PaaS
  kmm2        [PaaS] Display MirrorMaker2 info inside a PaaS
  ktopic      [PaaS] Display topics info inside a PaaS
  namespace   [PaaS] Display namespace info
  partition   [ERDING] Display the log dir info
  topic       [ERDING] Display topic info of a cluster
```

### Commands options

  * acl

Display acls of all or subset topics of a cluster

  * config

Display the config (static and dynamic) for the given cluster

        --null           Display the keys which have null value
    -n, --number int     Broker ID

  * topic

Display topic info of a cluster

    -d, --describe       Show the details of partitions

  * group

By default, get the list of groups (option --short) or the list of groups along with their state (default, no option) of the given clusters (clusters are comma separated).

If a group is passed (or several groups with comma separator), then describe, members and state are retrieved for the given group(s).

  * health

Used together with git, check the health of all clusters that are in the branch repository

Note : if no option is selected (like --urp or --umisr), then all options will be checked.

e.g. go run kstat.go --git_branch YOUR_BRANCH --git_login YOUR_LOGING --short health

    --amisr   Look only for at min in sync partitions
    --uav     Look only for partitions whose leader is unavailable
    --umisr   Look only for under min in sync partitions
    --urp     Look only for under replicated partitions

  * inventory

  Used together with the -c|--cluster option, restrains the inventory to the given cluster.

  e.g. go run kstat.go --git-branch ERDING_DEV --git-login jimbert -c bkt28 inventory

```
      --inventory-type string   Create the inventory for kafka, zookeeper or connect.  (default "kafka")
  -o, --outfile string          Output file name
      --stdin                   Write the inventory to stdin  (default true)
```

  * partition

  Pretty display with the --short|-s option, else raw display

    --broker-list string   The list of brokers to be queried in the form 0,1,2. All brokers in the cluster will be queried if no broker list is specified


### PaaS

  * kgroup

  [PaaS] Display groups info inside a PaaS

  * kmm2

  [PaaS] Display MirrorMaker2 info inside a PaaS

  * ktopic

  [PaaS] Display topics info inside a PaaS

  * namespace

  [PaaS] Display namespace info

### Global flags:

These options are available for all commands, but may not be used in some commands.

    -b, --broker string       Broker full name (e.g. bkuv1000.os.amadeus.net:9092)
    -c, --cluster string      Cluster name (e.g. bku10)
        --git-branch string   git branch to checkout (e.g. ERDING_TL1)
        --git-repo string     git repository to clone (default "https://rndwww.nce.amadeus.net/git/scm/kafka/ansible-configs.git")
    -g, --group string        Groups to describe (separator is comma for several groups)
    -h, --help                help for kstat
        --http-timeout int    Timeout used when sending a request (milliseconds) (default 2000)
        --inv string          Input ansible-like inventory file
        --kconfig string      Absolute path to the kubeconfig file
    -l, --log string          log level (e.g. trace, debug, info, warn, error, fatal) (default "warn")
    -u, --login string        login
        --ns string           Namespace names using comma as separator (e.g. namespace1,namespace2)
    -w, --passwd string       password
    -s, --short               When available, display only a short version of the results
        --timeout int         Timeout used when checking the connection (milliseconds) (default 500)
    -t, --topic string        Topic names using comma as separator (e.g. topic1,topic2)
