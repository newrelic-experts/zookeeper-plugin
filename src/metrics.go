package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/metric"
)

var metricsDefinition = map[string][]interface{}{
	"avg_latency":                {"zk_avg_latency", metric.GAUGE},
	"max_latency":                {"zk_max_latency", metric.GAUGE},
	"min_latency":                {"zk_min_latency", metric.GAUGE},
	"packets_received":           {"zk_packets_received", metric.GAUGE},
	"packets_sent":               {"zk_packets_sent", metric.GAUGE},
	"outstanding_requests":       {"zk_outstanding_requests", metric.GAUGE},
	"server_state":               {"zk_server_state", metric.ATTRIBUTE},
	"znode_count":                {"zk_znode_count", metric.GAUGE},
	"watch_count":                {"zk_watch_count", metric.GAUGE},
	"ephemerals_count":           {"zk_ephemerals_count", metric.GAUGE},
	"approximate_data_size":      {"zk_approximate_data_size", metric.GAUGE},
	"followers":                  {"zk_followers", metric.GAUGE},
	"synced_followers":           {"zk_synced_followers", metric.GAUGE},
	"pending_syncs":              {"zk_pending_syncs", metric.GAUGE},
	"open_file_descriptor_count": {"zk_open_file_descriptor_count", metric.GAUGE},
	"max_file_descriptor_count":  {"zk_max_file_descriptor_count", metric.GAUGE},
	"status":                     {"status", metric.GAUGE},
	"zk_host":                    {"zk_host", metric.ATTRIBUTE},
	"zk_port":                    {"zk_port", metric.ATTRIBUTE},
	"num_alive_connections":      {"zk_num_alive_connections", metric.GAUGE},
}

func asValue(value string) interface{} {
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}

	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}

	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	return value
}

func populateMetrics(sample *metric.MetricSet, metrics map[string]interface{}, metricsDefinition map[string][]interface{}) error {
	if len(metrics) == 0 {
		log.Debug("Metrics data from status module not found")
	}
	for metricName, metricInfo := range metricsDefinition {
		rawSource := metricInfo[0]
		metricType := metricInfo[1].(metric.SourceType)

		var rawMetric interface{}
		var ok bool

		switch source := rawSource.(type) {
		case string:
			rawMetric, ok = metrics[source]
		default:
			log.Warn("Invalid raw source metric for %s", metricName)
			continue
		}

		if !ok {
			log.Debug("Can't find raw metrics in results for %s [%s]", metricName, rawSource)
			continue
		}
		err := sample.SetMetric(metricName, rawMetric, metricType)

		if err != nil {
			log.Warn("Error setting value: %s", err)
			continue
		}
	}

	if len(*sample) < 2 {
		return fmt.Errorf("No metrics were found on the status response")
	}
	return nil
}

func checkNCExists(cmdExecutable string) {
	path, err := exec.LookPath(cmdExecutable)
	if err != nil {
		log.Error("%s executable not found in PATH\n", cmdExecutable)
	} else {
		log.Debug("%s executable is in '%s'\n", cmdExecutable, path)
	}
}

func runCommand(cmdExecutable string, zkCommand string) string {
	cmd := exec.Command(cmdExecutable, strings.TrimSpace(args.Host), strconv.Itoa(args.Port))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader(zkCommand)

	err := cmd.Run()
	if err != nil {
		log.Error("%s command failed with %s\n", cmdExecutable, err)
	}
	outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
	if errStr != "" {
		log.Debug("Errors running command:\n%s\n", errStr)
	}
	return outStr
}

func getMetricsData(sample *metric.MetricSet) error {
	cmdExecutable := strings.TrimSpace(args.Cmd)
	checkNCExists(cmdExecutable)

	rawMetrics := make(map[string]interface{})
	rawMetrics["zk_host"] = args.Host
	rawMetrics["zk_port"] = strconv.Itoa(args.Port)

	outStr := runCommand(cmdExecutable, "mntr")
	temp := strings.Split(outStr, "\n")
	for _, line := range temp {
		splitedLine := strings.Fields(line)
		if len(splitedLine) < 2 {
			continue
		}
		rawMetrics[splitedLine[0]] = asValue(strings.TrimSpace(splitedLine[1]))
	}

	outStr = runCommand(cmdExecutable, "ruok")
	if strings.Contains(outStr, "imok") {
		rawMetrics["status"] = 1
	} else {
		rawMetrics["status"] = 0
	}

	return populateMetrics(sample, rawMetrics, metricsDefinition)
}
