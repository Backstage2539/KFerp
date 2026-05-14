package contracts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	contractsapp "orderapp/internal/application/contracts"
	support "orderapp/internal/interfaces/http/support"
	"path"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

const maxContractHTTPUploadBytes = 30 << 20

type contractHandler struct {
	contracts ContractService
}

func registerContractRoutes(e *echo.Echo, contracts ContractService) {
	h := contractHandler{contracts: contracts}
	e.GET("/contracts", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, "/vue-shell?view=contracts"))
	})
	e.GET("/api/contracts", h.list)
	e.POST("/api/contracts", h.upload)
	e.GET("/contracts/:id/pdf", h.downloadPDF)
	e.POST("/api/contracts/:id/stamped", h.saveStamped)
	e.GET("/contracts/:id/stamped/:version_id.pdf", h.downloadStamped)
	e.GET("/contracts/:id/stamped-latest.pdf", h.downloadLatestStamped)
}

func (h contractHandler) list(c echo.Context) error {
	rows, err := h.contracts.ListContracts(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": rows})
}

func (h contractHandler) upload(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "file required"})
	}
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, maxContractHTTPUploadBytes+1))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	doc, err := h.contracts.UploadContract(c.Request().Context(), contractsapp.UploadContractCommand{
		Actor:       support.ActorOf(c),
		Filename:    file.Filename,
		ContentType: file.Header.Get(echo.HeaderContentType),
		Data:        data,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, doc)
}

func (h contractHandler) saveStamped(c echo.Context) error {
	contractID, err := parseContractID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "file required"})
	}
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, maxContractHTTPUploadBytes+1))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	sealAssetID, err := strconv.ParseInt(strings.TrimSpace(c.FormValue("seal_asset_id")), 10, 64)
	if err != nil || sealAssetID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "seal required"})
	}
	var placements []contractsapp.StampPlacement
	if err := json.Unmarshal([]byte(c.FormValue("placements")), &placements); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid placements"})
	}
	version, err := h.contracts.SaveStampedPDF(c.Request().Context(), contractsapp.SaveStampedPDFCommand{
		Actor:       support.ActorOf(c),
		ContractID:  contractID,
		SealAssetID: sealAssetID,
		Data:        data,
		Placements:  placements,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, version)
}

func (h contractHandler) downloadPDF(c echo.Context) error {
	contractID, err := parseContractID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	file, err := h.contracts.LoadContractPDFFile(c.Request().Context(), contractID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	return sendContractPDF(c, file)
}

func (h contractHandler) downloadStamped(c echo.Context) error {
	contractID, err := parseContractID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	versionID, err := parseContractVersionID(c.Param("version_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	file, err := h.contracts.LoadStampedPDFFile(c.Request().Context(), contractID, versionID, false)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	return sendContractPDF(c, file)
}

func (h contractHandler) downloadLatestStamped(c echo.Context) error {
	contractID, err := parseContractID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	file, err := h.contracts.LoadStampedPDFFile(c.Request().Context(), contractID, 0, true)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	return sendContractPDF(c, file)
}

func sendContractPDF(c echo.Context, file contractsapp.ContractFile) error {
	contentType := strings.TrimSpace(file.ContentType)
	if contentType == "" {
		contentType = "application/pdf"
	}
	filename := strings.TrimSpace(file.Filename)
	if filename == "" {
		filename = "contract.pdf"
	}
	c.Response().Header().Set(echo.HeaderContentType, contentType)
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.File(file.Path)
}

func parseContractID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid contract id")
	}
	return id, nil
}

func parseContractVersionID(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = path.Base(raw)
	}
	raw = strings.TrimSuffix(raw, ".pdf")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid version id")
	}
	return id, nil
}
