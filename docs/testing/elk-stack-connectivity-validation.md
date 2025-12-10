# ELK Stack Connectivity Validation

This document provides a checklist for validating the connectivity and basic functionality of the Elasticsearch, Logstash, and Kibana (ELK) stack. These checks are intended to be performed after deployment or a configuration change.

## 1. Elasticsearch Connectivity

| Check ID | Component | Description | Command / Action | Expected Result |
|---|---|---|---|---|
| ELK-V-001 | Elasticsearch | Verify the Elasticsearch host is reachable from a client machine. | `ping ${ELK_HOST}` | The host responds to pings. |
| ELK-V-002 | Elasticsearch | Check if the Elasticsearch REST API is responding on port 9200. | `curl http://${ELK_HOST}:9200` | The API returns a JSON response with cluster information (e.g., `cluster_name`, `version`). |
| ELK-V-003 | Elasticsearch | Perform a cluster health check. | `curl http://${ELK_HOST}:9200/_cluster/health` | The API returns a JSON response with `status: "green"` or `status: "yellow"`. |

## 2. Logstash Connectivity

| Check ID | Component | Description | Command / Action | Expected Result |
|---|---|---|---|---|
| ELK-V-004 | Logstash | Verify the TCP JSON input port (5000) is open. | `nc -z -v ${ELK_HOST} 5000` (or equivalent port scanning tool) | The connection is successful, indicating the port is open and listening. |
| ELK-V-005 | Logstash | Verify the Beats input port (5044) is open. | `nc -z -v ${ELK_HOST} 5044` | The connection is successful. |
| ELK-V-006 | Logstash | Send a test log entry via TCP. | `echo '{"message":"test"}' | nc ${ELK_HOST} 5000` | The command executes without error. The log should appear in Elasticsearch shortly after. |

## 3. Kibana Connectivity

| Check ID | Component | Description | Command / Action | Expected Result |
|---|---|---|---|---|
| ELK-V-007 | Kibana | Verify the Kibana UI is accessible on port 5601. | Open a web browser and navigate to `http://${ELK_HOST}:5601`. | The Kibana user interface loads successfully. |
| ELK-V-008 | Kibana | Check if Kibana can connect to Elasticsearch. | In the Kibana UI, navigate to "Stack Management" -> "Index Management". | The page loads and displays a list of Elasticsearch indices without connection errors. |

## 4. End-to-End Log Flow

| Check ID | Component | Description | Steps | Expected Result |
|---|---|---|---|---|
| ELK-V-009 | Full Stack | Verify that a log sent to Logstash is indexed in Elasticsearch and visible in Kibana. | 1. Send a test log with a unique message to Logstash (using the `nc` command from ELK-V-006). <br> 2. Wait a few moments for processing. <br> 3. In Kibana, go to the "Discover" tab and search for the unique message. | The test log entry is found and displayed in Kibana. |
| ELK-V-010 | Application | Verify that the Go application's logger can successfully ship logs to the stack. | 1. Configure a Go application instance with the correct `LOGSTASH_TCP_ADDR`. <br> 2. Trigger an action in the application that generates a log entry. <br> 3. Search for the log entry in Kibana. | The application log appears in Kibana. |

## 5. Network and Security

| Check ID | Component | Description | Action | Expected Result |
|---|---|---|---|---|
| ELK-V-011 | Network | Verify firewall rules allow access to required ports from trusted sources. | Review firewall configurations (e.g., `iptables`, cloud security groups). | Ports 9200, 5000, 5044, and 5601 are open to the IP ranges of application servers and admin users. Access from the public internet should be blocked unless explicitly required and secured. |
| ELK-V-012 | Network | Verify DNS resolution for `${ELK_HOST}`. | On a client machine, run `nslookup ${ELK_HOST}`. | The command resolves to the correct IP address of the ELK stack host. |
