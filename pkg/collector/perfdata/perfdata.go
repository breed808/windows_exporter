//go:build windows

package perfdata

import (
	"errors"
	"fmt"
	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus-community/windows_exporter/pkg/pdh"
	"github.com/prometheus-community/windows_exporter/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
)

const Name = "perfdata"

type Config struct{}

var ConfigDefaults = Config{}

// A collector is a Prometheus collector for WMI metrics
type collector struct {
	logger log.Logger

	perfCounters pdh.WinPerfCounters

	descriptors map[string]*prometheus.Desc
}

func New(logger log.Logger, _ *Config) types.Collector {
	c := &collector{}
	c.SetLogger(logger)
	return c
}

func NewWithFlags(_ *kingpin.Application) types.Collector {
	return &collector{}
}

func (c *collector) GetName() string {
	return Name
}

func (c *collector) SetLogger(logger log.Logger) {
	c.logger = log.With(logger, "collector", Name)
}

func (c *collector) GetPerfCounter() ([]string, error) {
	return []string{}, nil
}

func (c *collector) Build() error {
	c.perfCounters = pdh.WinPerfCounters{
		Log:                        c.logger,
		PrintValid:                 false,
		UsePerfCounterTime:         false,
		UseWildcardsExpansion:      true,
		LocalizeWildcardsExpansion: false,
		Object: []pdh.PerfObject{
			{
				WarnOnMissing: true,
				ObjectName:    "Processor Information",
				Instances:     []string{"*#1"},
				Counters: []string{
					"% Processor Performance",
				},
				UseRawValues: true,
			},
			{
				WarnOnMissing: true,
				ObjectName:    "Processor Information",
				Instances:     []string{"*#2"},
				Counters: []string{
					"% Processor Utility",
					"% Processor Performance",
				},
				UseRawValues: true,
			},
		},
	}

	_, err := c.perfCounters.Gather()
	if err != nil {
		return err
	}

	c.descriptors = make(map[string]*prometheus.Desc)

	c.descriptors["OSInformation"] = prometheus.NewDesc(
		prometheus.BuildFQName(types.Namespace, Name, "info"),
		"OperatingSystem.Caption, OperatingSystem.Version",
		[]string{"product", "version", "major_version", "minor_version", "build_number", "revision"},
		nil,
	)

	return nil
}

// Collect sends the metric values for each metric
// to the provided prometheus Metric channel.
func (c *collector) Collect(ctx *types.ScrapeContext, ch chan<- prometheus.Metric) error {
	if desc, err := c.collect(ctx, ch); err != nil {
		_ = level.Error(c.logger).Log("failed collecting os metrics", "desc", desc, "err", err)
		return err
	}
	return nil
}

func (c *collector) collect(_ *types.ScrapeContext, ch chan<- prometheus.Metric) (*prometheus.Desc, error) {
	acc, err := c.perfCounters.Gather()
	if err != nil {
		return nil, fmt.Errorf("failed to gather perf data: %w", err)
	}

	counter, ok := acc["localhost"]
	if !ok {
		return nil, errors.New("missing perf data")
	}

	processorCounter, ok := counter["Processor Information"]
	if !ok {
		return nil, errors.New("missing 'Processor Information' perf data")
	}

	for core, value := range processorCounter["C1 Transitions/sec"] {
		ch <- prometheus.MustNewConstMetric(
			c.CStateSecondsTotal,
			prometheus.CounterValue,
			value,
			core, "c1",
		)
	}

	for core, value := range processorCounter["C2 Transitions/sec"] {
		ch <- prometheus.MustNewConstMetric(
			c.CStateSecondsTotal,
			prometheus.CounterValue,
			value,
			core, "c2",
		)
	}

	for core, value := range processorCounter["C3 Transitions/sec"] {
		ch <- prometheus.MustNewConstMetric(
			c.CStateSecondsTotal,
			prometheus.CounterValue,
			value,
			core, "c3",
		)
	}

	for core, value := range processorCounter["% Idle Time"] {
		ch <- prometheus.MustNewConstMetric(
			c.TimeTotal,
			prometheus.CounterValue,
			value,
			core, "idle",
		)
	}

	for core, value := range processorCounter["% Interrupt Time"] {
		ch <- prometheus.MustNewConstMetric(
			c.TimeTotal,
			prometheus.CounterValue,
			value,
			core, "interrupt",
		)
	}

	for core, value := range processorCounter["% DPC Time"] {
		ch <- prometheus.MustNewConstMetric(
			c.TimeTotal,
			prometheus.CounterValue,
			value,
			core, "dpc",
		)
	}

	for core, value := range processorCounter["% Privileged Time"] {
		ch <- prometheus.MustNewConstMetric(
			c.TimeTotal,
			prometheus.CounterValue,
			value,
			core, "privileged",
		)
	}

	for core, value := range processorCounter["% User Time"] {
		ch <- prometheus.MustNewConstMetric(
			c.TimeTotal,
			prometheus.CounterValue,
			value,
			core, "user",
		)
	}

	for core, value := range processorCounter["Interrupts/sec"] {
		ch <- prometheus.MustNewConstMetric(
			c.InterruptsTotal,
			prometheus.CounterValue,
			value,
			core,
		)
	}

	for core, value := range processorCounter["DPCs Queued/sec"] {
		ch <- prometheus.MustNewConstMetric(
			c.DPCsTotal,
			prometheus.CounterValue,
			value,
			core,
		)
	}

	for core, value := range processorCounter["Clock Interrupts/sec"] {
		ch <- prometheus.MustNewConstMetric(
			c.ClockInterruptsTotal,
			prometheus.CounterValue,
			value,
			core,
		)
	}

	for core, value := range processorCounter["Idle Break Events/sec"] {
		ch <- prometheus.MustNewConstMetric(
			c.IdleBreakEventsTotal,
			prometheus.CounterValue,
			value,
			core,
		)
	}

	for core, value := range processorCounter["Parking Status"] {
		ch <- prometheus.MustNewConstMetric(
			c.ParkingStatus,
			prometheus.GaugeValue,
			value,
			core,
		)
	}

	for core, value := range processorCounter["Processor Frequency"] {
		ch <- prometheus.MustNewConstMetric(
			c.ProcessorFrequencyMHz,
			prometheus.GaugeValue,
			value,
			core,
		)
	}

	for core, value := range processorCounter["% Processor Performance"] {
		ch <- prometheus.MustNewConstMetric(
			c.ProcessorPerformance,
			prometheus.CounterValue,
			value,
			core,
		)
	}

	for core, value := range processorCounter["% Processor Utility"] {
		ch <- prometheus.MustNewConstMetric(
			c.ProcessorUtility,
			prometheus.CounterValue,
			value,
			core,
		)
	}

	for core, value := range processorCounter["% Privileged Utility"] {
		ch <- prometheus.MustNewConstMetric(
			c.ProcessorPrivUtility,
			prometheus.CounterValue,
			value,
			core,
		)
	}

	return nil, nil
}
