package roborock

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed product_catalog.json
var productCatalogJSON []byte

// ProductMetadata describes a known Roborock hardware model.
type ProductMetadata struct {
	Model       string `json:"model"`
	ProductName string `json:"product_name"`
	ProductID   string `json:"product_id"`
}

type catalogFile struct {
	Products map[string]struct {
		ProductName string `json:"product_name"`
		ProductID   string `json:"product_id"`
	} `json:"products"`
}

var (
	catalogOnce sync.Once
	catalogMap  map[string]ProductMetadata
)

func loadCatalog() map[string]ProductMetadata {
	catalogOnce.Do(func() {
		catalogMap = make(map[string]ProductMetadata)
		var parsed catalogFile
		if err := json.Unmarshal(productCatalogJSON, &parsed); err != nil {
			return
		}
		for model, entry := range parsed.Products {
			key := strings.ToLower(strings.TrimSpace(model))
			catalogMap[key] = ProductMetadata{
				Model:       model,
				ProductName: entry.ProductName,
				ProductID:   entry.ProductID,
			}
		}
	})
	return catalogMap
}

// LookupProduct resolves a Roborock model string (e.g. "roborock.vacuum.a144" or "a144")
// to its retail product name and catalog product ID.
func LookupProduct(model string) (ProductMetadata, bool) {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return ProductMetadata{}, false
	}
	cat := loadCatalog()
	if meta, ok := cat[m]; ok {
		return meta, true
	}
	if !strings.Contains(m, ".") {
		if meta, ok := cat["roborock.vacuum."+m]; ok {
			return meta, true
		}
	}
	return ProductMetadata{}, false
}

// DefaultProductName returns the friendly retail name for a model or a formatted fallback.
func DefaultProductName(model string) string {
	if meta, ok := LookupProduct(model); ok && meta.ProductName != "" {
		return meta.ProductName
	}
	m := strings.TrimSpace(model)
	if m == "" {
		return "Roborock Vacuum"
	}
	return m
}

// CameraBootstrapMode identifies which onboarding field a model uses for POST /fwapi/createca.
type CameraBootstrapMode string

const (
	// CameraBootstrapBoth indicates unknown/conservative checking of both token.r and country_domain.
	CameraBootstrapBoth CameraBootstrapMode = "both"
	// CameraBootstrapCountryDomain is used by Saros 10R (a144).
	CameraBootstrapCountryDomain CameraBootstrapMode = "country_domain"
	// CameraBootstrapTokenR is used by Saros 20 (a288) and Qrevo MaxV (a87).
	CameraBootstrapTokenR CameraBootstrapMode = "token.r"
)

// CameraBootstrapForModel returns the observed firmware camera bootstrap source for a model.
func CameraBootstrapForModel(model string) CameraBootstrapMode {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "roborock.vacuum.a144", "a144":
		return CameraBootstrapCountryDomain
	case "roborock.vacuum.a288", "a288", "roborock.vacuum.a87", "a87":
		return CameraBootstrapTokenR
	default:
		return CameraBootstrapBoth
	}
}
