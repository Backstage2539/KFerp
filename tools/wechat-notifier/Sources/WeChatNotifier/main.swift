import Foundation
#if SWIFT_PACKAGE
import NotifierCore
#endif
import AppKit
import ApplicationServices
import Security
import CryptoKit

// Profiles must come from observed, readable accessibility controls for a pinned
// WeChat version. There is deliberately no guessed profile or coordinate fallback.
struct Profile:Decodable {
 let version:String
 let titleID:String
 let groupIndicatorID:String
 let groupIndicatorValue:String
 let composerID:String
 let sendButtonID:String
 let historyID:String
 let sentMessageID:String
}
struct AgentError:Error,CustomStringConvertible {let description:String;init(_ s:String){description=s}}
func value(_ e:AXUIElement,_ key:String)->CFTypeRef? {var v:CFTypeRef?;return AXUIElementCopyAttributeValue(e,key as CFString,&v) == .success ? v : nil}
func textValue(_ e:AXUIElement)->String {(value(e,kAXValueAttribute) as? String) ?? (value(e,kAXTitleAttribute) as? String) ?? ""}
func children(_ e:AXUIElement)->[AXUIElement] {(value(e,kAXChildrenAttribute) as? [AXUIElement]) ?? []}
func descendants(_ e:AXUIElement)->[AXUIElement] {var result=[e],index=0;while index<result.count && result.count<4000{result += children(result[index]);index+=1};return result}
func unique(_ nodes:[AXUIElement],_ id:String)->AXUIElement? {guard !id.isEmpty else{return nil};let matches=nodes.filter{value($0,kAXIdentifierAttribute) as? String == id};return matches.count==1 ? matches[0]:nil}
struct Chat {
 let p:Profile;let root:AXUIElement;let nodes:[AXUIElement];let foreground:Bool
 init?(_ p:Profile){guard let app=NSRunningApplication.runningApplications(withBundleIdentifier:"com.tencent.xinWeChat").first,let url=app.bundleURL,let bundle=Bundle(url:url),bundle.object(forInfoDictionaryKey:"CFBundleVersion") as? String == p.version else{return nil};self.p=p;root=AXUIElementCreateApplication(app.processIdentifier);nodes=descendants(root);foreground=NSWorkspace.shared.frontmostApplication?.processIdentifier==app.processIdentifier}
 var locked:Bool {let session=CGSessionCopyCurrentDictionary() as? [String:Any];return session == nil || session?["CGSSessionScreenIsLocked"] as? Bool == true || session?["kCGSessionOnConsoleKey"] as? Bool == false}
 func snapshot()->ChatSnapshot {let title=unique(nodes,p.titleID),composer=unique(nodes,p.composerID),group=unique(nodes,p.groupIndicatorID);let readable=title != nil && composer != nil && unique(nodes,p.sendButtonID) != nil && unique(nodes,p.historyID) != nil && group.map{textValue($0)==p.groupIndicatorValue} == true && !p.sentMessageID.isEmpty;return ChatSnapshot(group:title.map{textValue($0)} ?? "",readable:readable,locked:locked,foreground:foreground,draft:composer.map{textValue($0)} ?? "")}
 func send(_ message:String,group:String)->String {
  guard snapshot().health(expected:group)=="ready",let editor=unique(nodes,p.composerID) else{return "failed"}
  var settable:DarwinBoolean=false
  guard AXUIElementIsAttributeSettable(editor,kAXValueAttribute as CFString,&settable) == .success,settable.boolValue else{return "failed"}
  guard AXUIElementSetAttributeValue(editor,kAXValueAttribute as CFString,message as CFString) == .success else{return "failed"}
  // Re-read both the target and draft immediately before sending.
  guard let fresh=Chat(p),fresh.foreground,!fresh.locked,let title=unique(fresh.nodes,p.titleID),textValue(title)==group,let draft=unique(fresh.nodes,p.composerID),textValue(draft)==message,let button=unique(fresh.nodes,p.sendButtonID) else{return "unknown"}
  guard AXUIElementPerformAction(button,kAXPressAction as CFString) == .success else{return "unknown"}
  for _ in 0..<12 {Thread.sleep(forTimeInterval:0.5);guard let current=Chat(p),let title=unique(current.nodes,p.titleID),textValue(title)==group,let history=unique(current.nodes,p.historyID) else{return "unknown"};let sent=descendants(history).filter{value($0,kAXIdentifierAttribute) as? String == p.sentMessageID}.map{textValue($0)};if deliveryResult(expected:message,messages:sent)=="delivered"{return "delivered"}}
  return "unknown"
 }
}
final class NoRedirect:NSObject,URLSessionTaskDelegate {
 func urlSession(_ session:URLSession,task:URLSessionTask,willPerformHTTPRedirection response:HTTPURLResponse,newRequest request:URLRequest,completionHandler:@escaping(URLRequest?)->Void){completionHandler(nil)}
}
struct API {
 let base:String;let token:String
 func post(_ path:String,_ body:[String:Any]) throws -> [String:Any]? {
  guard let url=URL(string:base+"/api/mini/notification-agent"+path) else{throw AgentError("服务器地址无效")}
  var request=URLRequest(url:url);request.httpMethod="POST";request.timeoutInterval=15;request.setValue("Bearer "+token,forHTTPHeaderField:"Authorization");request.setValue("application/json",forHTTPHeaderField:"Content-Type");request.httpBody=try JSONSerialization.data(withJSONObject:body)
  let wait=DispatchSemaphore(value:0);var responseData:Data?;var responseCode=0
  let session=URLSession(configuration:.ephemeral,delegate:NoRedirect(),delegateQueue:nil);defer{session.invalidateAndCancel()}
  let task=session.dataTask(with:request){data,response,_ in responseData=data;responseCode=(response as? HTTPURLResponse)?.statusCode ?? 0;wait.signal()};task.resume();if wait.wait(timeout:.now()+20) == .timedOut{task.cancel();throw AgentError("服务器连接超时，暂停发送")}
  if responseCode==204{return nil};guard responseCode==200,let data=responseData else{throw AgentError("服务器未接受请求（\(responseCode)），暂停发送")};return try JSONSerialization.jsonObject(with:data) as? [String:Any]
 }
}
func keychain(_ server:String,save:String?=nil)throws->String {
 let query:[String:Any]=[kSecClass as String:kSecClassGenericPassword,kSecAttrService as String:"KFerpWeChatNotifier",kSecAttrAccount as String:server]
 if let token=save {SecItemDelete(query as CFDictionary);var add=query;add[kSecValueData as String]=Data(token.utf8);add[kSecAttrAccessible as String]=kSecAttrAccessibleWhenUnlockedThisDeviceOnly;guard SecItemAdd(add as CFDictionary,nil)==errSecSuccess else{throw AgentError("无法保存到钥匙串")};return token}
 var find=query;find[kSecReturnData as String]=true;find[kSecMatchLimit as String]=kSecMatchLimitOne;var result:CFTypeRef?;guard SecItemCopyMatching(find as CFDictionary,&result)==errSecSuccess,let data=result as? Data,let token=String(data:data,encoding:.utf8) else{throw AgentError("请先配对设备")};return token
}
func run()throws {
 let args=CommandLine.arguments
 guard args.count>=3, ["development","production"].contains(args[2]) else{throw AgentError("用法：WeChatNotifier pair|run development|production [已验证的 profile.json]")}
 let base=args[2]=="development" ? "https://dev.qacoohee.com/app":"https://erp.qacoohee.com/app"
 if args[1]=="pair" {guard let raw=getpass("粘贴一次性配对凭据：" ) else{throw AgentError("未输入凭据")};let token=String(cString:raw);guard token.count==64,token.allSatisfy({$0.isHexDigit}) else{throw AgentError("配对凭据格式错误")};_ = try keychain(base,save:token);print("已存入钥匙串；尚未启动发送。");return}
 guard args[1]=="run",args.count==4 else{throw AgentError("缺少已验证的微信控件配置；当前版本默认不支持自动发送")}
 let profileData=try Data(contentsOf:URL(fileURLWithPath:args[3]));let profile=try JSONDecoder().decode(Profile.self,from:profileData);guard let executable=Bundle.main.executableURL else{throw AgentError("无法确认发送程序版本")}
 let fingerprint=SHA256.hash(data:profileData + (try Data(contentsOf:executable))).map{String(format:"%02x",$0)}.joined()
 let api=API(base:base,token:try keychain(base));var previous=""
 while true {
  do {
   let health = !AXIsProcessTrusted() ? "permission_missing" : (Chat(profile)?.snapshot().health(expected:previous) ?? "unsupported")
   guard let beat=try api.post("/heartbeat",["health":health,"fingerprint":fingerprint]),let group=beat["group_name"] as? String else{throw AgentError("设备未绑定群")}
   if group != previous {previous=group;continue}
   if health != "ready" {print("暂停：\(health)");Thread.sleep(forTimeInterval:10);continue}
   guard let result=try api.post("/claim",[:]),let job=result["job"] as? [String:Any],let id=job["id"] as? NSNumber,let lease=job["lease"] as? String,let message=result["text"] as? String,let target=result["group_name"] as? String else{Thread.sleep(forTimeInterval:3);continue}
   guard target==group else{throw AgentError("领取后目标发生变化，等待核对")}
   // The server records a lease before this operation. A crash or lost response
   // expires to unknown and blocks subsequent tasks, never automatic re-send.
   let status=Chat(profile)?.send(message,group:target) ?? "failed"
   _=try api.post("/jobs/\(id)/result",["lease":lease,"status":status,"evidence":status=="delivered" ? message:(status=="failed" ? "未按下发送，控件不可用":"发送结果不明，请核对通知号")])
   if status != "delivered" {print("暂停：请在通知配置中核对发送结果。")}
  } catch {print("暂停：\(error)")}
  Thread.sleep(forTimeInterval:5)
 }
}
do {try run()}catch {fputs("\(error)\n",stderr);exit(1)}
