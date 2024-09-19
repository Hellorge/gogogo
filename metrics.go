package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    "github.com/gizak/termui/v3"
    "github.com/gizak/termui/v3/widgets"
)

type RequestMetric struct {
    URL          string
    ResponseTime time.Duration
    MemoryUsed   uint64
    StatusCode   int
}

type ServerMetrics struct {
    TotalRequests    int64
    AverageResponse  time.Duration
    CacheSize        int32
    CacheHitRate     float64
    MemoryUsage      uint64
    ActiveGoroutines int32
}

func main() {
    if err := termui.Init(); err != nil {
        fmt.Printf("failed to initialize termui: %v", err)
        return
    }
    defer termui.Close()

    totalRequests := widgets.NewGauge()
    totalRequests.Title = "Total Requests"
    totalRequests.SetRect(0, 0, 50, 3)

    avgResponseTime := widgets.NewParagraph()
    avgResponseTime.Title = "Avg Response Time"
    avgResponseTime.SetRect(0, 3, 50, 6)

    memoryUsage := widgets.NewGauge()
    memoryUsage.Title = "Memory Usage"
    memoryUsage.SetRect(50, 0, 100, 3)

    activeGoroutines := widgets.NewParagraph()
    activeGoroutines.Title = "Active Goroutines"
    activeGoroutines.SetRect(50, 3, 100, 6)

    cacheInfo := widgets.NewParagraph()
    cacheInfo.Title = "Cache Info"
    cacheInfo.SetRect(0, 6, 100, 9)

    recentRequests := widgets.NewList()
    recentRequests.Title = "Recent Requests"
    recentRequests.SetRect(0, 9, 100, 30)

    draw := func() {
        termui.Render(totalRequests, avgResponseTime, memoryUsage, activeGoroutines, cacheInfo, recentRequests)
    }

    tickerCount := 0
    uiEvents := termui.PollEvents()
    ticker := time.NewTicker(time.Second).C

    for {
        select {
        case e := <-uiEvents:
            switch e.ID {
            case "q", "<C-c>":
                return
            }
        case <-ticker:
            serverMetrics := fetchServerMetrics()
            requestMetrics := fetchRequestMetrics()

            totalRequests.Percent = int(serverMetrics.TotalRequests % 100)
            totalRequests.Label = fmt.Sprintf("%d", serverMetrics.TotalRequests)

            avgResponseTime.Text = fmt.Sprintf("%.2fms", float64(serverMetrics.AverageResponse.Nanoseconds())/1e6)

            memoryUsage.Percent = int((serverMetrics.MemoryUsage * 100) / (1 << 30)) // Assumes 1GB max
            memoryUsage.Label = fmt.Sprintf("%dMB", serverMetrics.MemoryUsage/(1<<20))

            activeGoroutines.Text = fmt.Sprintf("%d", serverMetrics.ActiveGoroutines)

            cacheInfo.Text = fmt.Sprintf("Size: %d, Hit Rate: %.2f%%", serverMetrics.CacheSize, serverMetrics.CacheHitRate*100)

            recentRequestsText := []string{}
            for _, req := range requestMetrics {
                recentRequestsText = append(recentRequestsText, fmt.Sprintf("%s - %dms - %d", req.URL, req.ResponseTime.Milliseconds(), req.StatusCode))
            }
            recentRequests.Rows = recentRequestsText

            tickerCount++
            if tickerCount%5 == 0 {
                width, height := termui.TerminalDimensions()
                totalRequests.SetRect(0, 0, width/2, 3)
                avgResponseTime.SetRect(0, 3, width/2, 6)
                memoryUsage.SetRect(width/2, 0, width, 3)
                activeGoroutines.SetRect(width/2, 3, width, 6)
                cacheInfo.SetRect(0, 6, width, 9)
                recentRequests.SetRect(0, 9, width, height)
            }

            draw()
        }
    }
}

func fetchServerMetrics() ServerMetrics {
    resp, err := http.Get("http://localhost:8080/metrics/server")
    if err != nil {
        return ServerMetrics{}
    }
    defer resp.Body.Close()

    var metrics ServerMetrics
    json.NewDecoder(resp.Body).Decode(&metrics)
    return metrics
}

func fetchRequestMetrics() []RequestMetric {
    resp, err := http.Get("http://localhost:8080/metrics/requests")
    if err != nil {
        return nil
    }
    defer resp.Body.Close()

    var metrics []RequestMetric
    json.NewDecoder(resp.Body).Decode(&metrics)
    return metrics
}