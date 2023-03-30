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

func IsValidReadingType(val string) bool {
	switch val {
	case string(GeneralUsage), string(ControlledLoad), string(PrimaryExport), string(SecondaryExport):
		return true
	default:
		return false
	}
}

type HourlyReading struct {
	StartTime time.Time
	EndTime   time.Time
	EnergyKWh float64
	// The hourly reading can consist of multiple measurements with different quality methods and
	// reason codes/descriptions so we include them all here
	QualityMethod     []string
	ReasonCode        []int
	ReasonDescription []string
}

type UsageData map[NMI]map[ReadingType][]HourlyReading
