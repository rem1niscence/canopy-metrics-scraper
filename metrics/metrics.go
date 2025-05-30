package metrics

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

type MetricName string

const (
	BlockSize           MetricName = "canopy_block_size"
	DBPartitionTime     MetricName = "canopy_store_partition_time"
	BlockProcessingTime MetricName = "canopy_block_processing_time"
)

type Metric struct {
	Height         uint64
	BlockSize      uint64
	PartitionTime  float64
	BlockBuildTime float64
}

type MetricsManager struct {
	db         *sql.DB
	scrapURL   string
	metrics    map[MetricName]float64
	httpClient *http.Client
}

func New(dbFileName string, scrapURL string) (*MetricsManager, error) {
	db, err := sql.Open("sqlite3", dbFileName)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	metrics := &MetricsManager{
		db:       db,
		scrapURL: scrapURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	if err := metrics.createMetricsTable(); err != nil {
		return nil, err
	}

	return metrics, nil
}

func (m *MetricsManager) GetMetric(name MetricName) (float64, error) {
	if value, ok := m.metrics[name]; ok {
		return value, nil
	}
	return 0, fmt.Errorf("metric %s not found", name)
}

func (m *MetricsManager) Scrap() error {
	metricFamilies, err := m.retrieveMetrics()
	if err != nil {
		return err
	}

	// reset the metrics for each scrap
	m.metrics = make(map[MetricName]float64)
	for name, mf := range metricFamilies {
		switch mf.GetType() {
		case dto.MetricType_GAUGE:
			for _, metric := range mf.GetMetric() {
				switch MetricName(name) {
				case BlockSize:
					m.metrics[BlockSize] = metric.GetGauge().GetValue()
				case DBPartitionTime:
					m.metrics[DBPartitionTime] = metric.GetGauge().GetValue()
				case BlockProcessingTime:
					m.metrics[BlockProcessingTime] = metric.GetGauge().GetValue()
				}
			}
		}
	}

	return nil
}

func (m *MetricsManager) retrieveMetrics() (map[string]*dto.MetricFamily, error) {
	resp, err := m.httpClient.Get(m.scrapURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, err
	}

	return metricFamilies, nil
}

func (m *MetricsManager) InsertMetric(metric *Metric) error {
	var currBlockchainSize uint64
	err := m.db.QueryRow("SELECT COALESCE(SUM(block_size), 0) FROM metrics").Scan(&currBlockchainSize)
	if err != nil {
		return fmt.Errorf("failed to get current blockchain size: %w", err)
	}

	newBlockchainSize := currBlockchainSize + metric.BlockSize

	query := `
    INSERT INTO metrics (
        height,
        block_build_time,
        partition_time,
        block_size,
        blockchain_size
    ) VALUES (?, ?, ?, ?, ?)`

	_, err = m.db.Exec(query,
		metric.Height,
		metric.BlockBuildTime,
		metric.PartitionTime,
		metric.BlockSize,
		newBlockchainSize)

	if err != nil {
		return fmt.Errorf("failed to insert metric: %w", err)
	}

	return nil
}

func (m *MetricsManager) createMetricsTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS metrics (
    height INTEGER PRIMARY KEY,
    block_build_time REAL NOT NULL,
    partition_time REAL NOT NULL,
    block_size INTEGER NOT NULL,
    blockchain_size INTEGER NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    timestamp_delta_seconds INTEGER DEFAULT 0
);

    -- Trigger to calculate the timestamp delta in seconds
		CREATE TRIGGER IF NOT EXISTS calculate_timestamp_delta
		AFTER INSERT ON metrics
		BEGIN
		    UPDATE metrics
		    SET timestamp_delta_seconds = (
		        SELECT
		            CAST((julianday(NEW.timestamp) - julianday(prev.timestamp)) * 86400 AS INTEGER)
		        FROM metrics prev
		        WHERE prev.height = (
		            SELECT MAX(height)
		            FROM metrics
		            WHERE height < NEW.height
		        )
		    )
		    WHERE rowid = NEW.rowid;
		END;
	`
	_, err := m.db.Exec(query)
	return err
}
