package main

import (
	"fmt"

	jp "github.com/buger/jsonparser"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "eventstore"
	subsystem = ""
)

type exporter struct {
	up                 prometheus.Gauge
	processCPU         prometheus.Gauge
	processCPUScaled   prometheus.Gauge
	processMemoryBytes prometheus.Gauge
	diskIoReadBytes    prometheus.Counter
	diskIoWrittenBytes prometheus.Counter
	diskIoReadOps      prometheus.Counter
	diskIoWriteOps     prometheus.Counter
	uptimeSeconds      prometheus.Counter
	tcpSentBytes       prometheus.Counter
	tcpReceivedBytes   prometheus.Counter
	tcpConnections     prometheus.Gauge

	clientPendingSendBytes     *prometheus.GaugeVec
	clientPendingReceivedBytes *prometheus.GaugeVec
	clientTotalSendBytes       *prometheus.CounterVec
	clientTotalReceivedBytes   *prometheus.CounterVec

	queueLength         *prometheus.GaugeVec
	queueItemsProcessed *prometheus.CounterVec

	driveTotalBytes     *prometheus.GaugeVec
	driveAvailableBytes *prometheus.GaugeVec

	projectionRunning                     *prometheus.GaugeVec
	projectionProgress                    *prometheus.GaugeVec
	projectionEventsProcessedAfterRestart *prometheus.CounterVec

	clusterMemberAlive    *prometheus.GaugeVec
	clusterMemberIsMaster prometheus.Gauge
	clusterMemberIsSlave  prometheus.Gauge
	clusterMemberIsClone  prometheus.Gauge

	subscriptionTotalItemsProcessed      *prometheus.CounterVec
	subscriptionLastProcessedEventNumber *prometheus.GaugeVec
	subscriptionLastKnownEventNumber     *prometheus.GaugeVec
	subscriptionConnectionCount          *prometheus.GaugeVec
	subscriptionTotalInFlightMessages    *prometheus.GaugeVec
}

func newExporter() *exporter {
	return &exporter{
		up:                 createGauge("up", "Whether the EventStore scrape was successful"),
		processCPU:         createGauge("process_cpu", "Process CPU usage, 0 - number of cores"),
		processCPUScaled:   createGauge("process_cpu_scaled", "Process CPU usage scaled to number of cores, 0 - 1, 1 = full load on all cores"),
		processMemoryBytes: createGauge("process_memory_bytes", "Process memory usage, as reported by EventStore"),
		diskIoReadBytes:    createCounter("disk_io_read_bytes", "Total number of disk IO read bytes"),
		diskIoWrittenBytes: createCounter("disk_io_written_bytes", "Total number of disk IO written bytes"),
		diskIoReadOps:      createCounter("disk_io_read_ops", "Total number of disk IO read operations"),
		diskIoWriteOps:     createCounter("disk_io_write_ops", "Total number of disk IO write operations"),
		uptimeSeconds:      createCounter("uptime_seconds", "Total uptime seconds"),
		tcpSentBytes:       createCounter("tcp_sent_bytes", "TCP sent bytes"),
		tcpReceivedBytes:   createCounter("tcp_received_bytes", "TCP received bytes"),
		tcpConnections:     createGauge("tcp_connections", "Current number of TCP connections"),

		clientPendingSendBytes:     createItemGaugeVec("client_pending_send_bytes", "Consumer pending send bytes", []string{"client_name", "connection_id"}),
		clientPendingReceivedBytes: createItemGaugeVec("client_pending_received_bytes", "Consumer pending received bytes", []string{"client_name", "connection_id"}),

		clientTotalSendBytes:     createItemCounterVec("client_total_send_bytes", "Consumer total send bytes", []string{"client_name", "connection_id"}),
		clientTotalReceivedBytes: createItemCounterVec("client_total_received_bytes", "Consumer total received bytes", []string{"client_name", "connection_id"}),

		queueLength:         createItemGaugeVec("queue_length", "Queue length", []string{"queue"}),
		queueItemsProcessed: createItemCounterVec("queue_items_processed_total", "Total number items processed by queue", []string{"queue"}),

		driveTotalBytes:     createItemGaugeVec("drive_total_bytes", "Drive total size in bytes", []string{"drive"}),
		driveAvailableBytes: createItemGaugeVec("drive_available_bytes", "Drive available bytes", []string{"drive"}),

		projectionRunning:                     createItemGaugeVec("projection_running", "If 1, projection is in 'Running' state", []string{"projection"}),
		projectionProgress:                    createItemGaugeVec("projection_progress", "Projection progress 0 - 1, where 1 = projection progress at 100%", []string{"projection"}),
		projectionEventsProcessedAfterRestart: createItemCounterVec("projection_events_processed_after_restart_total", "Projection event processed count", []string{"projection"}),

		clusterMemberAlive:    createItemGaugeVec("cluster_member_alive", "If 1, cluster member is alive, as seen from current cluster member", []string{"member"}),
		clusterMemberIsMaster: createGauge("cluster_member_is_master", "If 1, current cluster member is the master"),
		clusterMemberIsSlave:  createGauge("cluster_member_is_slave", "If 1, current cluster member is a slave"),
		clusterMemberIsClone:  createGauge("cluster_member_is_clone", "If 1, current cluster member is a clone"),

		subscriptionTotalItemsProcessed:      createItemCounterVec("subscription_items_processed_total", "Total items processed by subscription", []string{"event_stream_id", "group_name"}),
		subscriptionLastProcessedEventNumber: createItemGaugeVec("subscription_last_processed_event_number", "Last event number processed by subscription", []string{"event_stream_id", "group_name"}),
		subscriptionLastKnownEventNumber:     createItemGaugeVec("subscription_last_known_event_number", "Last known event number in subscription", []string{"event_stream_id", "group_name"}),
		subscriptionConnectionCount:          createItemGaugeVec("subscription_connections", "Number of connections to subscription", []string{"event_stream_id", "group_name"}),
		subscriptionTotalInFlightMessages:    createItemGaugeVec("subscription_messages_in_flight", "Number of messages in flight for subscription", []string{"event_stream_id", "group_name"}),
	}
}

func (e *exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up.Desc()
	ch <- e.processCPU.Desc()
	ch <- e.processMemoryBytes.Desc()
	ch <- e.diskIoReadBytes.Desc()
	ch <- e.diskIoWrittenBytes.Desc()
	ch <- e.diskIoReadOps.Desc()
	ch <- e.diskIoWriteOps.Desc()
	ch <- e.uptimeSeconds.Desc()
	ch <- e.tcpSentBytes.Desc()
	ch <- e.tcpReceivedBytes.Desc()
	ch <- e.tcpConnections.Desc()

	e.queueLength.Describe(ch)
	e.queueItemsProcessed.Describe(ch)
	e.driveTotalBytes.Describe(ch)
	e.driveAvailableBytes.Describe(ch)
	e.projectionRunning.Describe(ch)
	e.projectionProgress.Describe(ch)
	e.projectionEventsProcessedAfterRestart.Describe(ch)

	if isInClusterMode() {
		e.clusterMemberAlive.Describe(ch)

		ch <- e.clusterMemberIsMaster.Desc()
		ch <- e.clusterMemberIsSlave.Desc()
		ch <- e.clusterMemberIsClone.Desc()
	}
}

func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	log.Info("Running scrape")

	if stats, err := getStats(); err != nil {
		log.WithError(err).Error("Error while getting data from EventStore")

		e.up.Set(0)
		ch <- e.up
	} else {
		e.up.Set(1)
		ch <- e.up

		e.processCPU.Set(getProcessCPU(stats))
		ch <- e.processCPU

		e.processCPUScaled.Set(getProcessCPUScaled(stats))
		ch <- e.processCPUScaled

		e.processMemoryBytes.Set(getProcessMemory(stats))
		ch <- e.processMemoryBytes

		e.diskIoReadBytes.Set(getDiskIoReadBytes(stats))
		ch <- e.diskIoReadBytes

		e.diskIoWrittenBytes.Set(getDiskIoWrittenBytes(stats))
		ch <- e.diskIoWrittenBytes

		e.diskIoReadOps.Set(getDiskIoReadOps(stats))
		ch <- e.diskIoReadOps

		e.diskIoWriteOps.Set(getDiskIoWriteOps(stats))
		ch <- e.diskIoWriteOps

		e.tcpConnections.Set(getTCPConnections(stats))
		ch <- e.tcpConnections

		e.tcpReceivedBytes.Set(getTCPReceivedBytes(stats))
		ch <- e.tcpReceivedBytes

		e.tcpSentBytes.Set(getTCPSentBytes(stats))
		ch <- e.tcpSentBytes

		e.clusterMemberIsMaster.Set(getIsMaster(stats))
		ch <- e.clusterMemberIsMaster

		e.clusterMemberIsSlave.Set(getIsSlave(stats))
		ch <- e.clusterMemberIsSlave

		e.clusterMemberIsClone.Set(getIsClone(stats))
		ch <- e.clusterMemberIsClone

		collectPerClientPendingSendBytesGauge(stats, e.clientPendingSendBytes, ch)
		collectPerClientPendingReceivedBytesGauge(stats, e.clientPendingReceivedBytes, ch)
		collectPerClientTotalSendBytesGauge(stats, e.clientTotalSendBytes, ch)
		collectPerClientTotalReceivedBytesGauge(stats, e.clientTotalReceivedBytes, ch)

		collectPerQueueGauge(stats, e.queueLength, getQueueLength, ch)
		collectPerQueueCounter(stats, e.queueItemsProcessed, getQueueItemsProcessed, ch)

		collectPerDriveGauge(stats, e.driveTotalBytes, getDriveTotalBytes, ch)
		collectPerDriveGauge(stats, e.driveAvailableBytes, getDriveAvailableBytes, ch)

		collectPerProjectionGauge(stats, e.projectionRunning, getProjectionIsRunning, ch)
		collectPerProjectionGauge(stats, e.projectionProgress, getProjectionProgress, ch)
		collectPerProjectionCounter(stats, e.projectionEventsProcessedAfterRestart, getProjectionEventsProcessedAfterRestart, ch)

		collectPerMemberGauge(stats, e.clusterMemberAlive, getMemberIsAlive, ch)

		collectPerSubscriptionCounter(stats, e.subscriptionTotalItemsProcessed, getSubscriptionTotalItemsProcessed, ch)
		collectPerSubscriptionGauge(stats, e.subscriptionConnectionCount, getSubscriptionConnectionCount, ch)
		collectPerSubscriptionGauge(stats, e.subscriptionLastKnownEventNumber, getSubscriptionLastKnownEventNumber, ch)
		collectPerSubscriptionGauge(stats, e.subscriptionLastProcessedEventNumber, getSubscriptionLastProcessedEventNumber, ch)
		collectPerSubscriptionGauge(stats, e.subscriptionTotalInFlightMessages, getSubscriptionTotalInFlightMessages, ch)
	}
}

func collectPerMemberGauge(stats *stats, vec *prometheus.GaugeVec, collectFunc func([]byte) float64, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ArrayEach(stats.gossipStats, func(value []byte, dataType jp.ValueType, offset int, err error) {
		externalHTTPIp, _ := jp.GetString(value, "externalHttpIp")
		externalHTTPPort, _ := jp.GetInt(value, "externalHttpPort")
		memberName := fmt.Sprintf("%s:%d", externalHTTPIp, externalHTTPPort)
		vec.WithLabelValues(memberName).Set(collectFunc(value))
	}, "members")

	vec.Collect(ch)
}

func getMemberIsAlive(member []byte) float64 {
	alive, _ := jp.GetBoolean(member, "isAlive")
	if alive {
		return 1
	}
	return 0
}

func collectPerProjectionGauge(stats *stats, vec *prometheus.GaugeVec, collectFunc func([]byte) float64, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ArrayEach(stats.projectionStats, func(value []byte, dataType jp.ValueType, offset int, err error) {
		projectionName, _ := jp.GetString(value, "effectiveName")
		vec.WithLabelValues(projectionName).Set(collectFunc(value))
	}, "projections")

	vec.Collect(ch)
}

func collectPerProjectionCounter(stats *stats, vec *prometheus.CounterVec, collectFunc func([]byte) float64, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ArrayEach(stats.projectionStats, func(value []byte, dataType jp.ValueType, offset int, err error) {
		projectionName, _ := jp.GetString(value, "effectiveName")
		vec.WithLabelValues(projectionName).Set(collectFunc(value))
	}, "projections")

	vec.Collect(ch)
}

func getProjectionIsRunning(projection []byte) float64 {
	status, _ := jp.GetString(projection, "status")
	if status == "Running" {
		return 1
	}
	return 0
}

func getProjectionProgress(projection []byte) float64 {
	progress, _ := jp.GetFloat(projection, "progress")
	return progress / 100.0 // scale to 0-1
}

func getProjectionEventsProcessedAfterRestart(projection []byte) float64 {
	processed, _ := jp.GetFloat(projection, "eventsProcessedAfterRestart")
	return processed
}

func collectPerClientPendingSendBytesGauge(stats *stats, vec *prometheus.GaugeVec, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ArrayEach(stats.tcpStats, func(value []byte, dataType jp.ValueType, offset int, err error) {
		pendingBytes, _ := jp.GetFloat(value, "pendingSendBytes")
		vec.WithLabelValues(getClientConnectionName(value), getConnectionID(value)).Set(pendingBytes)
	})

	vec.Collect(ch)
}

func collectPerClientPendingReceivedBytesGauge(stats *stats, vec *prometheus.GaugeVec, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ArrayEach(stats.tcpStats, func(value []byte, dataType jp.ValueType, offset int, err error) {
		pendingBytes, _ := jp.GetFloat(value, "pendingReceivedBytes")
		vec.WithLabelValues(getClientConnectionName(value), getConnectionID(value)).Set(pendingBytes)
	})

	vec.Collect(ch)
}

func collectPerClientTotalSendBytesGauge(stats *stats, vec *prometheus.CounterVec, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ArrayEach(stats.tcpStats, func(value []byte, dataType jp.ValueType, offset int, err error) {
		totalBytes, _ := jp.GetFloat(value, "totalBytesSent")
		vec.WithLabelValues(getClientConnectionName(value), getConnectionID(value)).Set(totalBytes)
	})

	vec.Collect(ch)
}

func collectPerClientTotalReceivedBytesGauge(stats *stats, vec *prometheus.CounterVec, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ArrayEach(stats.tcpStats, func(value []byte, dataType jp.ValueType, offset int, err error) {
		totalBytes, _ := jp.GetFloat(value, "totalBytesReceived")
		vec.WithLabelValues(getClientConnectionName(value), getConnectionID(value)).Set(totalBytes)
	})

	vec.Collect(ch)
}

func getClientConnectionName(stats []byte) string {
	value, _ := jp.GetString(stats, "clientConnectionName")
	return value
}

func getConnectionID(stats []byte) string {
	value, _ := jp.GetString(stats, "connectionId")
	return value
}

func collectPerQueueGauge(stats *stats, vec *prometheus.GaugeVec, collectFunc func([]byte) float64, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ObjectEach(stats.serverStats, func(key []byte, value []byte, dataType jp.ValueType, offset int) error {
		queueName := string(key)
		vec.WithLabelValues(queueName).Set(collectFunc(value))
		return nil
	}, "es", "queue")

	vec.Collect(ch)
}

func collectPerQueueCounter(stats *stats, vec *prometheus.CounterVec, collectFunc func([]byte) float64, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ObjectEach(stats.serverStats, func(key []byte, value []byte, dataType jp.ValueType, offset int) error {
		queueName := string(key)
		vec.WithLabelValues(queueName).Set(collectFunc(value))
		return nil
	}, "es", "queue")

	vec.Collect(ch)
}

func getQueueLength(queue []byte) float64 {
	value, _ := jp.GetFloat(queue, "length")
	return value
}

func getQueueItemsProcessed(queue []byte) float64 {
	value, _ := jp.GetFloat(queue, "totalItemsProcessed")
	return value
}

func collectPerDriveGauge(stats *stats, vec *prometheus.GaugeVec, collectFunc func([]byte) float64, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ObjectEach(stats.serverStats, func(key []byte, value []byte, dataType jp.ValueType, offset int) error {
		drive := string(key)
		vec.WithLabelValues(drive).Set(collectFunc(value))
		return nil
	}, "sys", "drive")

	vec.Collect(ch)
}

func getDriveTotalBytes(drive []byte) float64 {
	value, _ := jp.GetFloat(drive, "totalBytes")
	return value
}

func getDriveAvailableBytes(drive []byte) float64 {
	value, _ := jp.GetFloat(drive, "availableBytes")
	return value
}

func collectPerSubscriptionCounter(stats *stats, vec *prometheus.CounterVec, collectFunc func([]byte) float64, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ArrayEach(stats.subscriptionsStats, func(value []byte, dataType jp.ValueType, offset int, err error) {
		eventStreamID, _ := jp.GetString(value, "eventStreamId")
		groupName, _ := jp.GetString(value, "groupName")
		vec.WithLabelValues(eventStreamID, groupName).Set(collectFunc(value))
	})

	vec.Collect(ch)
}

func collectPerSubscriptionGauge(stats *stats, vec *prometheus.GaugeVec, collectFunc func([]byte) float64, ch chan<- prometheus.Metric) {

	// Reset before collection to ensure we remove items that have been deleted
	vec.Reset()

	jp.ArrayEach(stats.subscriptionsStats, func(value []byte, dataType jp.ValueType, offset int, err error) {
		eventStreamID, _ := jp.GetString(value, "eventStreamId")
		groupName, _ := jp.GetString(value, "groupName")
		vec.WithLabelValues(eventStreamID, groupName).Set(collectFunc(value))
	})

	vec.Collect(ch)
}

func getSubscriptionTotalItemsProcessed(subscription []byte) float64 {
	value, _ := jp.GetFloat(subscription, "totalItemsProcessed")
	return value
}

func getSubscriptionConnectionCount(subscription []byte) float64 {
	value, _ := jp.GetFloat(subscription, "connectionCount")
	return value
}

func getSubscriptionLastProcessedEventNumber(subscription []byte) float64 {
	value, _ := jp.GetFloat(subscription, "lastProcessedEventNumber")
	return value
}

func getSubscriptionLastKnownEventNumber(subscription []byte) float64 {
	value, _ := jp.GetFloat(subscription, "lastKnownEventNumber")
	return value
}

func getSubscriptionTotalInFlightMessages(subscription []byte) float64 {
	value, _ := jp.GetFloat(subscription, "totalInFlightMessages")
	return value
}

func getProcessCPU(stats *stats) float64 {
	value, _ := jp.GetFloat(stats.serverStats, "proc", "cpu")
	return value / 100.0
}

func getProcessCPUScaled(stats *stats) float64 {
	value, _ := jp.GetFloat(stats.serverStats, "proc", "cpuScaled")
	return value / 100.0
}

func getProcessMemory(stats *stats) float64 {
	value, _ := jp.GetFloat(stats.serverStats, "proc", "mem")
	return value
}

func getDiskIoReadBytes(stats *stats) float64 {
	value, _ := jp.GetFloat(stats.serverStats, "proc", "diskIo", "readBytes")
	return value
}

func getDiskIoWrittenBytes(stats *stats) float64 {
	value, _ := jp.GetFloat(stats.serverStats, "proc", "diskIo", "writtenBytes")
	return value
}

func getDiskIoReadOps(stats *stats) float64 {
	value, _ := jp.GetFloat(stats.serverStats, "proc", "diskIo", "readOps")
	return value
}

func getDiskIoWriteOps(stats *stats) float64 {
	value, _ := jp.GetFloat(stats.serverStats, "proc", "diskIo", "writeOps")
	return value
}

func getTCPSentBytes(stats *stats) float64 {
	value, _ := jp.GetFloat(stats.serverStats, "proc", "tcp", "sentBytesTotal")
	return value
}

func getTCPReceivedBytes(stats *stats) float64 {
	value, _ := jp.GetFloat(stats.serverStats, "proc", "tcp", "receivedBytesTotal")
	return value
}

func getTCPConnections(stats *stats) float64 {
	value, _ := jp.GetFloat(stats.serverStats, "proc", "tcp", "connections")
	return value
}

func getIsMaster(stats *stats) float64 {
	return getIs("master", stats)
}

func getIsSlave(stats *stats) float64 {
	return getIs("slave", stats)
}

func getIsClone(stats *stats) float64 {
	return getIs("clone", stats)
}

func getIs(status string, stats *stats) float64 {
	value, _ := jp.GetString(stats.info, "state")
	if value == status {
		return 1
	}
	return 0
}

func createGauge(name string, help string) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	})
}

func createItemGaugeVec(name string, help string, itemLabels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}, itemLabels)
}

func createCounter(name string, help string) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	})
}

func createItemCounterVec(name string, help string, itemLabels []string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}, itemLabels)
}

func isInClusterMode() bool {
	return clusterMode == "cluster"
}
