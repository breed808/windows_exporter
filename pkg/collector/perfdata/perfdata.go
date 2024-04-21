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
	"strings"
)

const Name = "perfdata"

type Config struct{}

var ConfigDefaults = Config{}

// A collector is a Prometheus collector for WMI metrics
type collector struct {
	logger log.Logger

	perfCounters pdh.WinPerfCounters

	descriptors map[string]map[string]*prometheus.Desc
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
		UsePerfCounterTime:         true,
		UseWildcardsExpansion:      true,
		LocalizeWildcardsExpansion: true,
		Object: []pdh.PerfObject{
			{
				FailOnMissing: true,
				ObjectName:    "Processor Information",
				Instances:     []string{"*"},
				Counters:      []string{"% Processor Utility"},
				UseRawValues:  true,
				IncludeTotal:  false,
			},
			//{
			//	FailOnMissing: true,
			//	ObjectName:    "LogicalDisk",
			//	Instances:     []string{"*"},
			//	Counters:      []string{"*"},
			//	UseRawValues:  true,
			//	IncludeTotal:  false,
			//},
			//{
			//	FailOnMissing: true,
			//	ObjectName:    "PhysicalDisk",
			//	Instances:     []string{"*"},
			//	Counters:      []string{"*"},
			//	UseRawValues:  true,
			//	IncludeTotal:  false,
			//},
		},
	}

	acc, err := c.perfCounters.Gather()
	if err != nil {
		return fmt.Errorf("failed to gather perf data: %w", err)
	}

	counters, ok := acc["localhost"]
	if !ok {
		return errors.New("missing perf data")
	}

	c.descriptors = map[string]map[string]*prometheus.Desc{}
	for objectName, objectCounters := range counters {
		subSystem := sanitizeMetricName(objectName)

		c.descriptors[objectName] = map[string]*prometheus.Desc{}
		for instanceName, _ := range objectCounters {
			name := sanitizeMetricName(instanceName)
			c.descriptors[objectName][instanceName] = prometheus.NewDesc(
				prometheus.BuildFQName(types.Namespace+"_perfdata", subSystem, name),
				fmt.Sprintf("Perf counter %s @ %s", objectName, instanceName),
				[]string{"instance"},
				nil,
			)
		}
	}

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

	counters, ok := acc["localhost"]
	if !ok {
		return nil, errors.New("missing perf data")
	}

	for objectName, objectCounters := range counters {
		for instanceName, instanceCounter := range objectCounters {
			for instance, value := range instanceCounter {
				ch <- prometheus.MustNewConstMetric(
					c.descriptors[objectName][instanceName],
					prometheus.CounterValue,
					value,
					instance,
				)
			}
		}
	}

	return nil, nil
}

func sanitizeMetricName(name string) string {
	return strings.ReplaceAll(
		strings.TrimSpace(
			strings.ReplaceAll(
				strings.ReplaceAll(
					strings.ReplaceAll(
						strings.ToLower(name),
						".", "",
					),
					"%", "",
				),
				"/", "_",
			),
		),
		" ", "_",
	)
}
