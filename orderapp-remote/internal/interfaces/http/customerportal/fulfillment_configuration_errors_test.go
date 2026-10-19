package customerportal

import (
	"encoding/json"
	"errors"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	"testing"
)

func TestMiniFulfillmentConfigurationErrorsRemainActionable(t *testing.T) {
	for _, message := range []string{"请选择已开通的代发场景", "代发费用已变化，请重新预览后提交", "留厂目标仓必须为当前客户绑定的库存代发仓", "商品下单不能使用代发场景", "运费模板未覆盖当前收件地区", "商品包装数量或净重未配置，无法计算运费", "该场景尚未配置代发服务费", "代发不能设置销售留厂交付方式", "留厂购货请按 BOM 库存单位 袋 填写数量，暂不支持销售包装单位换算"} {
		e := echo.New()
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest("POST", "/api/mini/direct-ship/preview", nil), rec)
		if err := miniCustomerFulfillmentError(c, errors.New(message)); err != nil {
			t.Fatal(err)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if rec.Code >= 500 || body["error"] != message {
			t.Errorf("message=%s status=%d body=%s", message, rec.Code, rec.Body.String())
		}
	}
}
