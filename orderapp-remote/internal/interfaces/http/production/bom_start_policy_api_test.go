package production
import("context";"fmt";"testing";"encoding/json";app "orderapp/internal/application/production")
func TestNewPlanFreezesBOMStartPolicyWithoutPerMaterialNodes(t *testing.T){
 pool,schema:=newProductionFlowTestDB(t);ctx:=context.Background()
 server,_,work:=createSubmittedFullStockTypedPlan(t,ctx,pool,schema,74790,"bom_start_v1")
 var policy string;var consumed int64
 if err:=pool.QueryRow(ctx,fmt.Sprintf(`SELECT material_consumption_policy FROM %s.work_orders WHERE id=$1`,schema),work).Scan(&policy);err!=nil{t.Fatal(err)}
 if policy!="bom_start_v1"{t.Fatalf("new order policy=%s",policy)}
 if err:=pool.QueryRow(ctx,fmt.Sprintf(`SELECT COALESCE(SUM(consumed_g+consumed_units),0) FROM %s.work_order_material_reservations WHERE work_order_id=$1`,schema),work).Scan(&consumed);err!=nil{t.Fatal(err)}
 if consumed!=0{t.Fatal("planning consumed stock")}
 mustExecProductionFlowTestSQL(t,ctx,pool,fmt.Sprintf(`UPDATE %s.materials SET onhand_g=22700 WHERE id=10`,schema))
 var card int64;if err:=pool.QueryRow(ctx,fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY sequence_no,id LIMIT 1`,schema),work).Scan(&card);err!=nil{t.Fatal(err)}
 var p app.OperationCompletionPreview
 preview:=func(){t.Helper();res:=serveMultilevelProductionJSON(t,server,"GET",fmt.Sprintf("/api/job-cards/%d/start-preview",card),nil);if res.Code!=200{t.Fatal(res.Body.String())};if err:=json.Unmarshal(res.Body.Bytes(),&p);err!=nil{t.Fatal(err)}}
 preview()
 if p.BlockedReason!=""||len(p.Materials)!=2{t.Fatalf("bad preview %+v",p)}
 started:=serveMultilevelProductionJSON(t,server,"POST",fmt.Sprintf("/api/job-cards/%d/start",card),map[string]any{"actual_input_qty":p.InputQty,"input_unit":p.InputUnit,"materials":p.Materials,"expected_version":p.Version,"request_id":"ordinary-plan-start"})
 if started.Code!=200{t.Fatal(started.Body.String())}
 preview()
 done:=serveMultilevelProductionJSON(t,server,"POST",fmt.Sprintf("/api/job-cards/%d/complete",card),map[string]any{"actual_input_qty":p.InputQty,"actual_output_qty":p.PlannedQty,"input_unit":p.InputUnit,"output_unit":p.OutputUnit,"expected_version":p.Version,"request_id":"ordinary-plan-complete"})
 if done.Code!=200{t.Fatal(done.Body.String())}
 assertProductionFlowCount(t,pool,schema,"production_started_inputs",fmt.Sprintf("work_order_id=%d AND remaining_qty=0 AND remaining_cost=0",work),1)
 assertProductionFlowCount(t,pool,schema,"stock_entries",fmt.Sprintf("work_order_id=%d AND entry_type='finished_receipt' AND status='submitted'",work),1)

}
