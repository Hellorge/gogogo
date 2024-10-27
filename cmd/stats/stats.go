package main

import (
	"fmt"
	"gogogo/modules/metrics"
	"log"
	"time"

	"github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

type MetricsUI struct {
	cpuChart      *widgets.Plot
	memChart      *widgets.Plot
	reqChart      *widgets.Plot
	gauges        []*widgets.Gauge
	requestList   *widgets.List
	summaryText   *widgets.Paragraph
	profilerStats *widgets.Paragraph
}

func NewMetricsUI() *MetricsUI {
	return &MetricsUI{
		cpuChart:      widgets.NewPlot(),
		memChart:      widgets.NewPlot(),
		reqChart:      widgets.NewPlot(),
		gauges:        make([]*widgets.Gauge, 3),
		requestList:   widgets.NewList(),
		summaryText:   widgets.NewParagraph(),
		profilerStats: widgets.NewParagraph(),
	}
}

func (m *MetricsUI) setupUI() {
	termui.Clear()

	m.cpuChart.Title = "CPU Usage"
	m.cpuChart.LineColors = []termui.Color{termui.ColorGreen}
	m.cpuChart.AxesColor = termui.ColorWhite
	m.cpuChart.Data = make([][]float64, 1)

	m.memChart.Title = "Memory Usage"
	m.memChart.LineColors = []termui.Color{termui.ColorYellow}
	m.memChart.AxesColor = termui.ColorWhite
	m.memChart.Data = make([][]float64, 1)

	m.reqChart.Title = "Request Rate"
	m.reqChart.LineColors = []termui.Color{termui.ColorCyan}
	m.reqChart.AxesColor = termui.ColorWhite
	m.reqChart.Data = make([][]float64, 1)

	for i := range m.gauges {
		m.gauges[i] = widgets.NewGauge()
		m.gauges[i].BarColor = termui.ColorBlue
	}
	m.gauges[0].Title = "Cache Hit Rate"
	m.gauges[1].Title = "Memory Usage"
	m.gauges[2].Title = "Active Goroutines"

	m.requestList.Title = "Recent Requests"
	m.requestList.TextStyle = termui.NewStyle(termui.ColorWhite)
	m.requestList.WrapText = false

	m.summaryText.Title = "Summary"
	m.summaryText.TextStyle = termui.NewStyle(termui.ColorWhite)

	m.profilerStats.Title = "Profiler Stats"
	m.profilerStats.TextStyle = termui.NewStyle(termui.ColorWhite)

	m.layoutUI()
}

func (m *MetricsUI) layoutUI() {
	termWidth, termHeight := termui.TerminalDimensions()

	chartHeight := termHeight / 3
	gaugeHeight := 3
	summaryHeight := 5

	// Charts
	m.cpuChart.SetRect(0, 0, termWidth/3, chartHeight)
	m.memChart.SetRect(termWidth/3, 0, 2*termWidth/3, chartHeight)
	m.reqChart.SetRect(2*termWidth/3, 0, termWidth, chartHeight)

	// Gauges
	gaugeWidth := termWidth / 3
	for i, gauge := range m.gauges {
		gauge.SetRect(i*gaugeWidth, chartHeight, (i+1)*gaugeWidth, chartHeight+gaugeHeight)
	}

	// Summary
	m.summaryText.SetRect(0, chartHeight+gaugeHeight, termWidth, chartHeight+gaugeHeight+summaryHeight)

	// Request List
	m.requestList.SetRect(0, chartHeight+gaugeHeight+summaryHeight, termWidth, termHeight)

	profilerStatsHeight := 5
	m.profilerStats.SetRect(0, termHeight-profilerStatsHeight, termWidth, termHeight)
}

func (m *MetricsUI) updateCharts() {
	matr := metrics.GetMetrics()
	serverMetrics := matr.GetServerMetrics()

	cpuUsage := float64(serverMetrics.ActiveGoroutines) / 100.0 // This is a placeholder. You might want to implement actual CPU usage tracking.
	memUsage := float64(serverMetrics.MemoryUsage) / (1024 * 1024)
	reqRate := float64(serverMetrics.TotalRequests) // This should be requests per second. You might want to implement a rolling average.

	if len(m.cpuChart.Data[0]) >= 100 {
		m.cpuChart.Data[0] = m.cpuChart.Data[0][1:]
	}
	m.cpuChart.Data[0] = append(m.cpuChart.Data[0], cpuUsage)

	if len(m.memChart.Data[0]) >= 100 {
		m.memChart.Data[0] = m.memChart.Data[0][1:]
	}
	m.memChart.Data[0] = append(m.memChart.Data[0], memUsage)

	if len(m.reqChart.Data[0]) >= 100 {
		m.reqChart.Data[0] = m.reqChart.Data[0][1:]
	}
	m.reqChart.Data[0] = append(m.reqChart.Data[0], reqRate)
}

func (m *MetricsUI) updateGauges() {
	matr := metrics.GetMetrics()
	serverMetrics := matr.GetServerMetrics()

	m.gauges[0].Percent = int(float64(serverMetrics.CacheHitRate) / 1e4)                         // Convert from 1e6 fixed-point to percentage
	m.gauges[1].Percent = int((float64(serverMetrics.MemoryUsage) / (1024 * 1024 * 1024)) * 100) // Assuming 1GB max
	m.gauges[2].Percent = int((float64(serverMetrics.ActiveGoroutines) / 1000) * 100)            // Assuming 1000 max
}

func (m *MetricsUI) updateRequestList() {
	matr := metrics.GetMetrics()
	requestMetrics := matr.GetRequestMetrics()

	items := make([]string, 0, len(requestMetrics))
	for _, req := range requestMetrics {
		if req.URL != "" { // Only add non-empty entries
			items = append(items, fmt.Sprintf("%s - %dms - %d", req.URL, req.ResponseTime.Milliseconds(), req.StatusCode))
		}
	}
	m.requestList.Rows = items
}

func (m *MetricsUI) updateSummary() {
	matr := metrics.GetMetrics()
	serverMetrics := matr.GetServerMetrics()

	m.summaryText.Text = fmt.Sprintf(
		"Total Requests: %d\nAverage Response Time: %.2fms\nCache Size: %d\nMemory Usage: %.2f MB",
		serverMetrics.TotalRequests,
		float64(serverMetrics.AverageResponse)/float64(time.Millisecond),
		serverMetrics.CacheSize,
		float64(serverMetrics.MemoryUsage)/(1024*1024),
	)
}

func (m *MetricsUI) updateProfilerStats(stats profiler.ProfileStats) {
	m.profilerStats.Text = fmt.Sprintf(
		"Uptime: %v\nAllocated Memory: %v MB\nTotal Allocations: %v MB\nSystem Memory: %v MB\nGC Runs: %d",
		stats.Uptime.Round(time.Second),
		stats.AllocatedMem/1024/1024,
		stats.TotalAlloc/1024/1024,
		stats.Sys/1024/1024,
		stats.NumGC,
	)
}

func (m *MetricsUI) Run(profiler *profiler.Profiler) error {
	if err := termui.Init(); err != nil {
		return fmt.Errorf("failed to initialize termui: %v", err)
	}
	defer termui.Close()

	m.setupUI()

	updateUI := func() {
		m.updateCharts()
		m.updateGauges()
		m.updateRequestList()
		m.updateSummary()
		m.updateProfilerStats(profiler.GetStats())
	}

	updateUI()

	uiEvents := termui.PollEvents()
	ticker := time.NewTicker(time.Second).C

	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return nil
			case "<Resize>":
				m.layoutUI()
				termui.Clear()
				updateUI()
				termui.Render(m.cpuChart, m.memChart, m.reqChart, m.gauges[0], m.gauges[1], m.gauges[2], m.summaryText, m.requestList, m.profilerStats)
			}
		case <-ticker:
			updateUI()
			termui.Render(m.cpuChart, m.memChart, m.reqChart, m.gauges[0], m.gauges[1], m.gauges[2], m.summaryText, m.requestList, m.profilerStats)
		}
	}
}

func main() {
	profiler := profiler.NewProfiler()
	err := profiler.Start("../meta/cpu.prof", "../meta/mem.prof")
	if err != nil {
		log.Fatalf("Failed to start profiler: %v", err)
	}
	defer profiler.Stop()

	ui := NewMetricsUI()
	if err := ui.Run(profiler); err != nil {
		log.Fatalf("Error running ui: %v", err)
	}
}
