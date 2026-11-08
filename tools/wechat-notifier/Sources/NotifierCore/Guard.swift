public struct ChatSnapshot {
 public var group:String; public var readable:Bool; public var locked:Bool; public var foreground:Bool; public var draft:String
 public init(group:String,readable:Bool,locked:Bool,foreground:Bool,draft:String){self.group=group;self.readable=readable;self.locked=locked;self.foreground=foreground;self.draft=draft}
 public func health(expected:String)->String {
  if locked{return "locked"};if !readable{return "unsupported"};if group != expected{return "target_mismatch"};if !foreground || !draft.isEmpty{return "paused"};return "ready"
 }
}
public func deliveryResult(expected:String,messages:[String])->String {messages.contains(expected) ? "delivered" : "unknown"}
