### TPS Competition Questionnaire

*Please replace the square brackets and the text in it with your answers*

**Number of CPUs**

[How many logical CPUs does the machine have for running one cluster?]

**Memory (GB)**

[How much memory does the machine have for running one cluster? For example, 16G.]

**Storage (GB)**

[Note down both the type and capacity of the storage on one machine. For example, SSD 1024G.]

**Network**

[Comment on the network connecting clusters. For example, 1 Gbps LAN]

**Machine Type (Optional)**

[If you are using public cloud service, note down the name of the provider and the machine type. For example, AWS EC2 m5.2xlarge.]

**Command Lines for Running Cluster**
```
[Copy the command line here]
```

**Peak TPS**

[Note down the top 2 highest TPS observed. The final peak TPS value will be the smaller ones in the TOP TWO clusters’(at least 3 clusters) peak TPS.]

**Video URL**

[URL for the video showing how you produced the above TPS. Also, the submitted video should include stats of the observations of at least two clusters’ peak TPS.]

**Output From `stats` Tool (Highest TPS)**
```
[Copy the complete output from `stats` here. It should capture the cluster configuration and at least 10 minute continuous samples (60 rows) with at least one sample showing the heightest TPS claimed above.]
```
**Output From `stats` Tool (Second highest TPS)**
```
[Copy the complete output from `stats` here. It should capture the cluster configuration and at least 10 minute continuous samples (60 rows) with at least one sample showing the second highest TPS claimed above.]
```

**Cluster Configurations**
[Provide the link to the cluster config file (cluster_config_template.json, checked in to your forked repo) used in your clusters. 
If deployer is used, please provide deployConfig.json as well (password is not needed).]

**Additional Comment**

[If you have special setup, e.g., running a single cluster over multiple machines, the above questionnaire might not fit. Note down
whatever you want us to know here to help evaluate the result.]