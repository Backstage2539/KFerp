package customerportal
import (
 "testing"
 app "orderapp/internal/application/customerportal"
)
func TestPriceSampleIdentityFromSnapshot(t *testing.T){
 p:=app.BeanListSummary{ID:10}
 err:=parseBeanListDisplaySummary([]byte(`{}`),[]byte(`{"groups":[{"items":[{"name":"同名","product_id":3,"parent_product_id":2},{"name":"同名","product_id":5},{"name":"旧展示"}]}]}`),&p)
 if err!=nil{t.Fatal(err)}
 a,b,c:=p.Groups[0].Items[0],p.Groups[0].Items[1],p.Groups[0].Items[2]
 if a.ProductID!=2||b.ProductID!=5||a.SampleKey==b.SampleKey||a.SampleKey==""||c.SampleKey!=""{t.Fatalf("identity %+v",p.Groups)}
}
