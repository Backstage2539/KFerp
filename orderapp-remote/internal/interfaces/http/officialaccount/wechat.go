package officialaccount

import "orderapp/internal/infrastructure/wechataccount"

type Config = wechataccount.Config
type WechatClient = wechataccount.WechatClient

var ConfigFromEnv = wechataccount.ConfigFromEnv
