package iso

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/prefix"
)

type SpecJSON struct {
	Version       string                       `json:"version"`
	Description   string                       `json:"description"`
	Fields        map[string]FieldSpec         `json:"fields"`
	ResponseCodes map[string]string            `json:"responseCodes"`
}

type FieldSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Length      int    `json:"length"`
	Format      string `json:"format"`
	Encoding    string `json:"encoding"`
	Padding     string `json:"padding"`
	PadChar     string `json:"padChar"`
}

// LoadSpecFromJSON loads ISO 8583 spec from JSON file and creates MessageSpec
func LoadSpecFromJSON(path string) (*iso8583.MessageSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file: %w", err)
	}

	var specJSON SpecJSON
	if err := json.Unmarshal(data, &specJSON); err != nil {
		return nil, fmt.Errorf("failed to parse spec JSON: %w", err)
	}

	fields := make(map[int]field.Field)

	// Field 0 - MTI
	fields[0] = field.NewString(&field.Spec{
		Length:      4,
		Description: "MTI",
		Enc:         encoding.ASCII,
		Pref:        prefix.ASCII.Fixed,
	})

	// Field 1 - Bitmap
	fields[1] = field.NewBitmap(&field.Spec{
		Description: "Bitmap",
		Enc:         encoding.Binary,
		Pref:        prefix.ASCII.Fixed,
	})

	// Convert JSON fields to moov-io field specs
	for fieldNumStr, fieldSpec := range specJSON.Fields {
		if fieldNumStr == "0" {
			continue // Already handled MTI
		}

		var fieldNum int
		fmt.Sscanf(fieldNumStr, "%d", &fieldNum)

		if fieldNum < 2 {
			continue
		}

		// Determine prefix based on format
		var pref prefix.Prefixer
		switch fieldSpec.Format {
		case "fixed":
			pref = prefix.ASCII.Fixed
		case "llvar":
			pref = prefix.ASCII.LL
		case "lllvar":
			pref = prefix.ASCII.LLL
		default:
			pref = prefix.ASCII.Fixed
		}

		// Create field
		fields[fieldNum] = field.NewString(&field.Spec{
			Length:      fieldSpec.Length,
			Description: fieldSpec.Description,
			Enc:         encoding.ASCII,
			Pref:        pref,
		})
	}

	return &iso8583.MessageSpec{
		Fields: fields,
	}, nil
}

// InitSpec initializes the global Spec variable from spec.json
func InitSpec(specPath string) error {
	loadedSpec, err := LoadSpecFromJSON(specPath)
	if err != nil {
		return err
	}
	Spec = loadedSpec
	return nil
}
