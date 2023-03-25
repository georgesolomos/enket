package nem12

import (
	"time"
)

// Header record (100)
type HeaderRecord struct {
	VersionHeader   string
	DateTime        time.Time
	FromParticipant string
	ToParticipant   string
}

// NMI data details record (200)
type NMIDataDetailsRecord struct {
	NMI                     string
	NMIConfiguration        string
	RegisterID              string
	NMISuffix               string
	MDMDataStreamIdentifier string
	MeterSerialNumber       string
	UOM                     string
	IntervalLength          int
	NextScheduledReadDate   time.Time
}

// Interval data record (300)
type IntervalDataRecord struct {
	IntervalDate      time.Time
	IntervalValues    []float64
	QualityMethod     string
	ReasonCode        int
	ReasonDescription string
	UpdateDateTime    time.Time
	MSATSLoadDateTime time.Time
}

// Interval event record (400)
type IntervalEventRecord struct {
	StartInterval     int
	EndInterval       int
	QualityMethod     string
	ReasonCode        int
	ReasonDescription string
}

// B2B details record (500)
type B2bDetailsRecord struct {
	TransCode       string
	RetServiceOrder string
	ReadDataTime    time.Time
	IndexRead       string
}
