package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

type ServerMetrics struct {
	TotalRequests    int64
	AverageResponse  time.Duration
	CacheSize        int32
	CacheHitRate     float64
	MemoryUsage      uint64
	ActiveGoroutines int32
}

type RequestMetric struct {
	URL          string
	ResponseTime time.Duration
	MemoryUsed   uint64
	StatusCode   int
}

type MetricsUI struct {
	serverMetrics  ServerMetrics
	recentRequests []RequestMetric
	cpuChart       *widgets.Plot
	memChart       *widgets.Plot
	reqChart       *widgets.Plot
	gauges         []*widgets.Gauge
	requestList    *widgets.List
	summaryText    *widgets.Paragraph
}

func NewMetricsUI() *MetricsUI {
	return &MetricsUI{
		cpuChart:    widgets.NewPlot(),
		memChart:    widgets.NewPlot(),
		reqChart:    widgets.NewPlot(),
		gauges:      make([]*widgets.Gauge, 3),
		requestList: widgets.NewList(),
		summaryText: widgets.NewParagraph(),
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

	m.layoutUI()
}

func (m *MetricsUI) layoutUI() {
	termWidth, termHeight := termui.TerminalDimensions()

	chartHeight := termHeight / 3
	gaugeHeight := 3
	summaryHeight := 5
	// listHeight := termHeight - chartHeight - gaugeHeight - summaryHeight

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

	// Request List (now at the bottom, full width)
	m.requestList.SetRect(0, chartHeight+gaugeHeight+summaryHeight, termWidth, termHeight)
}

// ... [Keep other methods (updateCharts, updateGauges, etc.) as they were] ...

func (m *MetricsUI) Run() error {
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
				// payload := e.Payload.(termui.Resize)
				m.layoutUI()
				termui.Clear()
				updateUI()
				termui.Render(m.cpuChart, m.memChart, m.reqChart, m.gauges[0], m.gauges[1], m.gauges[2], m.summaryText, m.requestList)
			}
		case <-ticker:
			m.fetchServerMetrics()
			m.fetchRequestMetrics()
			updateUI()
			termui.Render(m.cpuChart, m.memChart, m.reqChart, m.gauges[0], m.gauges[1], m.gauges[2], m.summaryText, m.requestList)
		}
	}
}

func (m *MetricsUI) updateCharts() {
	cpuUsage := float64(m.serverMetrics.ActiveGoroutines) / 100.0 // Placeholder
	memUsage := float64(m.serverMetrics.MemoryUsage) / (1024 * 1024)
	reqRate := float64(m.serverMetrics.TotalRequests) // This should be requests per second

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
	m.gauges[0].Percent = int(m.serverMetrics.CacheHitRate * 100)
	m.gauges[1].Percent = int((float64(m.serverMetrics.MemoryUsage) / (1024 * 1024 * 1024)) * 100) // Assuming 1GB max
	m.gauges[2].Percent = int((float64(m.serverMetrics.ActiveGoroutines) / 1000) * 100)            // Assuming 1000 max
}

func (m *MetricsUI) updateRequestList() {
	items := make([]string, len(m.recentRequests))
	for i, req := range m.recentRequests {
		items[i] = fmt.Sprintf("%s - %dms - %d", req.URL, req.ResponseTime.Milliseconds(), req.StatusCode)
	}
	m.requestList.Rows = items
}

func (m *MetricsUI) updateSummary() {
	m.summaryText.Text = fmt.Sprintf(
		"Total Requests: %d\nAverage Response Time: %.2fms\nCache Size: %d\nMemory Usage: %.2f MB",
		m.serverMetrics.TotalRequests,
		float64(m.serverMetrics.AverageResponse)/float64(time.Millisecond),
		m.serverMetrics.CacheSize,
		float64(m.serverMetrics.MemoryUsage)/(1024*1024),
	)
}

func (m *MetricsUI) fetchMetrics() {
	for {
		m.fetchServerMetrics()
		m.fetchRequestMetrics()
		termui.Render(m.cpuChart, m.memChart, m.reqChart, m.gauges[0], m.gauges[1], m.gauges[2], m.requestList, m.summaryText)
		time.Sleep(1 * time.Second)
	}
}

func (m *MetricsUI) fetchServerMetrics() {
	resp, err := http.Get("http://localhost:8080/metrics/server")
	if err != nil {
		log.Println("Error fetching server metrics:", err)
		return
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&m.serverMetrics)
	if err != nil {
		log.Println("Error decoding server metrics:", err)
	}
}

func (m *MetricsUI) fetchRequestMetrics() {
	resp, err := http.Get("http://localhost:8080/metrics/requests")
	if err != nil {
		log.Println("Error fetching request metrics:", err)
		return
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&m.recentRequests)
	if err != nil {
		log.Println("Error decoding request metrics:", err)
	}
}

func main() {
	ui := NewMetricsUI()
	if err := ui.Run(); err != nil {
		log.Fatalf("error running ui: %v", err)
	}
}
