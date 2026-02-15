package internal

import (
	"fmt"
	"io"
)

// metricsWriter writes metrics from a group of stores to an io.Writer.
type metricsWriter struct {
	stores []*StoreType
}

// newMetricsWriter creates a new metricsWriter.
func newMetricsWriter(stores ...*StoreType) *metricsWriter {
	return &metricsWriter{
		stores: stores,
	}
}

// writeStores writes out metrics from the underlying stores to the given writer, per resource.
// It writes metrics so that the ones with the same name are grouped together when written out, and guarantees an exposition format that is safe to be ingested by Prometheus.
func (m *metricsWriter) writeStores(writer io.Writer) error {
	if len(m.stores) == 0 {
		return nil
	}

	for _, store := range m.stores {
		store.mutex.RLock()
		err := m.writeFromStore(writer, store)
		store.mutex.RUnlock()

		if err != nil {
			return err
		}
	}

	return nil
}

func (m *metricsWriter) writeFromStore(writer io.Writer, store *StoreType) error {
	for i, header := range store.headers {
		if err := writeHeader(writer, header); err != nil {
			return fmt.Errorf("error writing header: %w", err)
		}

		for _, metricFamilies := range store.metrics {
			if i >= len(metricFamilies) {
				continue
			}
			if err := writeMetricFamily(writer, metricFamilies[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeHeader(writer io.Writer, header string) error {
	if header != "" && header != "\n" {
		header += "\n"
	}
	_, err := writer.Write([]byte(header))

	return err
}

func writeMetricFamily(writer io.Writer, metric string) error {
	n, err := writer.Write([]byte(metric))
	if err != nil {
		return fmt.Errorf("error writing metric family after %d bytes: %w", n, err)
	}

	return nil
}
