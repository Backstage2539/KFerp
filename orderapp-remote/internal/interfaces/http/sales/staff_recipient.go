package sales

import (
	"encoding/json"
	"io"
	"net/http"

	fulfillmentapp "orderapp/internal/application/customerfulfillment"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

// The compatibility endpoint uses the full fulfillment transaction so a changed
// destination cannot leave the saved freight quote attached to the old address.
func (h orderAPIHandler) staffRecipient(c echo.Context) error {
	id, ok, err := h.staffFulfillmentAccess(c)
	if !ok {
		return err
	}

	var req struct {
		CustomerID      int64   `json:"customer_id"`
		EditRevision    string  `json:"edit_revision"`
		ReceiverName    string  `json:"receiver_name"`
		ReceiverPhone   string  `json:"receiver_phone"`
		ReceiverAddress string  `json:"receiver_address"`
		ReceiverCompany *string `json:"receiver_company"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "仅支持修改收件人、电话、地址和公司"})
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "无效收件信息"})
	}
	cmd := fulfillmentapp.FulfillmentOrderEditCommand{
		OrderID: id, CustomerID: req.CustomerID, Actor: support.ActorOf(c), RecipientOnly: true,
		ReceiverName: req.ReceiverName, ReceiverPhone: req.ReceiverPhone,
		ReceiverAddress: req.ReceiverAddress, ReceiverCompany: req.ReceiverCompany,
		ExpectedEditRevision: req.EditRevision,
	}
	result, err := h.fulfillmentEditor.UpdateFulfillmentOrder(c.Request().Context(), cmd)
	if err != nil {
		return fulfillmentEditError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}
