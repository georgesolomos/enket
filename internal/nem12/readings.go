package nem12

import "time"

// National Meter Identifier. Unique for each connection point.
type NMI string

// Describes the type of interval data the record applies to. Mapped from the NMI suffix.
type ReadingType string

// We interpret NMI suffixes in a broad, generalised way that will be mostly correct for our purposes
const (
	GeneralUsage    ReadingType = "E1"
	ControlledLoad  ReadingType = "E2"
	PrimaryExport   ReadingType = "B1"
	SecondaryExport ReadingType = "B2"
)

type EnergyUnit string

const (
	MWh EnergyUnit = "MWH"
	KWh EnergyUnit = "KWH"
	Wh  EnergyUnit = "WH"
)

type UsageData struct {
	NMI            NMI
	NMISuffix      ReadingType
	HourlyReadings []HourlyReading
}

type HourlyReading struct {
	StartTime time.Time
	EndTime   time.Time
	// We don't support reactive measurements (e.g. kVarh). They're not as applicable to consumer
	// power usage and billing, and we'd need the power factor to calculate kWh.
	Energy            float64
	QualityMethod     string
	ReasonCode        int
	ReasonDescription string
}
