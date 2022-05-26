# KafkaStatus

This CLI GO tool is aimed at gathering information from a Kafka cluster.

Available Commands:

  acl         Display acls of all or subset topics of a cluster
    -t, --topic string   Topic names using comma as separator (e.g. topic1,topic2)

  config      Display the config (static and dynamic) for the given cluster
    -u, --null           Display the keys which have null value
    -n, --number int     Broker ID)
    -s, --synonym        Display the keys which correspond to synonym as well

  topic       Display topic info of a cluster
    -a, --all            Check all health options
    -s, --sum            Display only a summary of the output
    -t, --topic string   Topic names using comma as separator (e.g. topic1,topic2)
    --amisr              Look for at min in sync partitions
    --uav                Look for partitions whose leader is unavailable
    --umisr              Look for under min in sync partitions
    --urp                Look for under replicated partitions

Flags:
  -b, --broker string    Broker full name (e.g. bkuv1000.os.amadeus.net:9092)
  -c, --cluster string   Cluster name (e.g. bku10)
  -h, --help             help for KafkaStatus
