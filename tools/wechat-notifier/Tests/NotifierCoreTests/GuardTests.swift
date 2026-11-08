func XCTAssertEqual<T:Equatable>(_ actual:T,_ expected:T){precondition(actual==expected,"expected \(expected), got \(actual)")}
@main struct GuardTests {
 static func main(){testFailsClosedForUnreadableWrongLockedAndDraft();testOnlyExactReadbackCanConfirmAndNoSuccessOnSilence();print("PASS 9 notification guard assertions")}
 static func testFailsClosedForUnreadableWrongLockedAndDraft() {
  var s=ChatSnapshot(group:"测试生产群", readable:true, locked:false, foreground:true, draft:"")
  XCTAssertEqual(s.health(expected:"测试生产群"),"ready")
  s.group="其他群";XCTAssertEqual(s.health(expected:"测试生产群"),"target_mismatch")
  s.group="测试生产群";s.readable=false;XCTAssertEqual(s.health(expected:"测试生产群"),"unsupported")
  s.readable=true;s.locked=true;XCTAssertEqual(s.health(expected:"测试生产群"),"locked")
  s.locked=false;s.draft="未发送草稿";XCTAssertEqual(s.health(expected:"测试生产群"),"paused")
  s.draft="";s.foreground=false;XCTAssertEqual(s.health(expected:"测试生产群"),"paused")
 }
 static func testOnlyExactReadbackCanConfirmAndNoSuccessOnSilence() {
  XCTAssertEqual(deliveryResult(expected:"测试\n通知号：N1", messages:["测试\n通知号：N1"]),"delivered")
  XCTAssertEqual(deliveryResult(expected:"测试\n通知号：N1", messages:["测试\n通知号：N2"]),"unknown")
  XCTAssertEqual(deliveryResult(expected:"测试\n通知号：N1", messages:[]),"unknown")
 }
}
