package nem12

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"golang.org/x/exp/slog"
)

type Parser struct {
	logger *slog.Logger
	file   *os.File
}

func NewParser(logger *slog.Logger, file *os.File) *Parser {
	return &Parser{
		logger: logger,
		file:   file,
	}
}

func (p *Parser) Parse() {
	nemReader := p.createNemReader(p.file)

	hasHeader, err := p.checkNem12Header(nemReader)
	if err != nil {
		p.logger.Error(err.Error())
		os.Exit(1)
	}

	if !hasHeader {
		// Reset the file and recreate the reader to start reading from the first row again
		_, err = p.file.Seek(0, io.SeekStart)
		if err != nil {
			p.logger.Error(err.Error())
			os.Exit(1)
		}
		nemReader = p.createNemReader(p.file)
	}

	records := make(chan []string)
	go p.getNem12Records(nemReader, records)
	// Specifies the rules for the 300-500 blocks to be read for the 200 record
	var currentDetails *NMIDataDetailsRecord
	for record := range records {
		if len(record) == 0 {
			continue
		}
		p.logger.Debug(fmt.Sprintf("%v", record))
		recordIndicator, err := strconv.Atoi(record[0])
		if err != nil {
			p.logger.Warn("Could not parse record indicator - moving to next line")
		}
		switch recordIndicator {
		case 200: // Data details
			currentDetails, err = p.parse200Record(record)
			if err != nil {
				p.logger.Error(err.Error())
				return
			}
			p.logger.Info("Parsed 200 record", slog.Any("record", currentDetails))
		case 300: // Interval data
			data, err := p.parse300Record(record, currentDetails)
			if err != nil {
				p.logger.Error(err.Error())
				return
			}
			p.logger.Info("Parsed 300 record", slog.Any("record", data))
		case 400: // Interval event
			data, err := p.parse400Record(record, currentDetails)
			if err != nil {
				p.logger.Error(err.Error())
				return
			}
			p.logger.Info("Parsed 400 record", slog.Any("record", data))
		case 900: // End of data
			return
		}
	}
}

func (p *Parser) parse200Record(record []string) (*NMIDataDetailsRecord, error) {
	// Mandatory field
	intervalLength, err := strconv.Atoi(record[8])
	if err != nil {
		return nil, errors.New("interval length cannot be parsed")
	}

	// Optional field
	nextScheduledReadDate := time.Time{}
	if record[9] != "" {
		nextScheduledReadDate, err = time.Parse("20060102", record[9])
		if err != nil {
			// If we can't parse the date, log it and move on since the field is optional anyway
			p.logger.Warn("parse200Record: next scheduled read date cannot be parsed")
		}
	}

	return &NMIDataDetailsRecord{
		NMI:                     record[1],
		NMIConfiguration:        record[2],
		RegisterID:              record[3],
		NMISuffix:               record[4],
		MDMDataStreamIdentifier: record[5],
		MeterSerialNumber:       record[6],
		UOM:                     record[7],
		IntervalLength:          intervalLength,
		NextScheduledReadDate:   nextScheduledReadDate,
	}, nil
}

func (p *Parser) parse300Record(record []string, currentDetails *NMIDataDetailsRecord) (*IntervalDataRecord, error) {
	// Mandatory field
	intervalDate, err := time.Parse("20060102", record[1])
	if err != nil {
		return nil, errors.New("interval date cannot be parsed")
	}

	// As per the spec, "The number of values provided must equal 1440 divided by the IntervalLength"
	// There are 2 other mandatory fields,
	numIntervalVals := 1440 / currentDetails.IntervalLength
	idxAfterIntervalVals := numIntervalVals + 2
	if len(record) < idxAfterIntervalVals {
		return nil, errors.New("not enough interval values")
	}

	vals := []float64{}
	for _, val := range record[2 : numIntervalVals+2] {
		valFlt, err := strconv.ParseFloat(val, 64)
		if err != nil {
			// This isn't great but we continue in case it's a one-off mistake
			p.logger.Warn("parse300Record: interval value cannot be parsed")
			valFlt = 0
		}
		vals = append(vals, valFlt)
	}

	reasonCode, err := strconv.Atoi(record[idxAfterIntervalVals+1])
	if err != nil {
		p.logger.Warn("parse300Record: reason code cannot be parsed")
	}

	return &IntervalDataRecord{
		IntervalDate:      intervalDate,
		IntervalValues:    vals,
		QualityMethod:     record[idxAfterIntervalVals],
		ReasonCode:        reasonCode,
		ReasonDescription: record[idxAfterIntervalVals+2],
	}, nil
}

func (p *Parser) parse400Record(record []string, currentDetails *NMIDataDetailsRecord) (*IntervalEventRecord, error) {
	return &IntervalEventRecord{}, nil
}

func (p *Parser) checkNem12Header(nemReader *csv.Reader) (bool, error) {
	record, err := nemReader.Read()
	if err != nil {
		return false, err
	}
	if record[0] != "100" {
		p.logger.Info("No header record - assuming NEM12 format")
		return false, nil
	}
	if record[1] != "NEM12" {
		return true, errors.New("header record indicates this is not a NEM12 file")
	}
	return true, nil
}

func (p *Parser) getNem12Records(csvReader *csv.Reader, records chan<- []string) {
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			p.logger.Error(err.Error())
		}
		records <- record
	}
	close(records)
}

func (p *Parser) createNemReader(file *os.File) *csv.Reader {
	csvReader := csv.NewReader(file)
	// NEM12 files have variable fields per record, so we tell the CSV reader to not expect
	// any particular number
	csvReader.FieldsPerRecord = -1
	return csvReader
}
