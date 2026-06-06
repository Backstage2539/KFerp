package customerportal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"
	messagecenterapp "orderapp/internal/application/messagecenter"
	salesapp "orderapp/internal/application/sales"
	pdfinfra "orderapp/internal/infrastructure/pdf"

	"github.com/labstack/echo/v4"
)

type Service interface {
	Login(context.Context, customerportalapp.LoginCommand) (customerportalapp.LoginResult, error)
	LoginWithPassword(context.Context, customerportalapp.PasswordLoginCommand) (customerportalapp.LoginResult, error)
	Me(context.Context, string) (customerportalapp.CurrentContext, error)
	SwitchCurrentCustomer(context.Context, string, int64) (customerportalapp.CurrentContext, error)
	GetServicePage(context.Context, string, string, customerportalapp.ServicePageFilter) (customerportalapp.ServicePage, error)
	GetBeanListPublication(context.Context, string, int64) (customerportalapp.BeanListSummary, error)
	AcknowledgeBeanListPublication(context.Context, string, int64) error
	GetResaleBeanLists(context.Context, string) (customerportalapp.ResaleBeanListPage, error)
	GetCustomerProducts(context.Context, string) (customerportalapp.CustomerProductsPage, error)
	CreateCustomerProductCategory(context.Context, string, customerportalapp.CustomerProductCategoryCommand) (customerportalapp.CustomerProductCategory, error)
	UpdateCustomerProductCategory(context.Context, string, int64, customerportalapp.CustomerProductCategoryCommand) (customerportalapp.CustomerProductCategory, error)
	DeleteCustomerProductCategory(context.Context, string, int64) error
	MoveCustomerProductCategory(context.Context, string, int64, customerportalapp.CustomerProductCategoryMoveCommand) (customerportalapp.CustomerProductCategory, error)
	AssignCustomerProductCategory(context.Context, string, int64, customerportalapp.CustomerProductCategoryAssignmentCommand) (customerportalapp.CustomerProductSummary, error)
	GetResaleBeanListEditor(context.Context, string, int64) (customerportalapp.ResaleBeanListEditor, error)
	SaveResaleBeanListDraft(context.Context, string, customerportalapp.ResaleBeanListCommand) (customerportalapp.BeanListSummary, error)
	PublishResaleBeanList(context.Context, string, customerportalapp.ResaleBeanListCommand) (customerportalapp.BeanListSummary, error)
	GetResaleBeanListPublicationPDF(context.Context, string, int64, func(customerportalapp.BeanListSummary) ([]byte, error)) (customerportalapp.BeanListSummary, []byte, error)
	GetResaleBeanListPublicationPNG(context.Context, string, int64, func(customerportalapp.BeanListSummary) ([]byte, error)) (customerportalapp.BeanListSummary, []byte, error)
	ListPortalAdminCustomers(context.Context, customerportalapp.PortalAdminCustomerQuery) ([]customerportalapp.PortalAdminCustomer, error)
	PortalAdminDetail(context.Context, int64) (customerportalapp.PortalAdminDetail, error)
	UpdatePortalVisibility(context.Context, customerportalapp.UpdatePortalVisibilityCommand) (customerportalapp.PortalAdminDetail, error)
	ListCapabilityTemplates(context.Context) ([]customerportalapp.CapabilityTemplate, error)
	SaveCapabilityTemplate(context.Context, customerportalapp.SaveCapabilityTemplateCommand) (customerportalapp.CapabilityTemplate, error)
	CopyCapabilityTemplate(context.Context, customerportalapp.CopyCapabilityTemplateCommand) (customerportalapp.CapabilityTemplate, error)
	ApplyCapabilityTemplate(context.Context, customerportalapp.ApplyCapabilityTemplateCommand) (customerportalapp.PortalAdminDetail, error)
	UpsertPortalERPBinding(context.Context, customerportalapp.UpsertPortalERPBindingCommand) (customerportalapp.PortalAdminDetail, error)
	ListMallProducts(context.Context) ([]customerportalapp.MallProduct, []customerportalapp.MallProductOption, error)
	SaveMallProduct(context.Context, customerportalapp.SaveMallProductCommand) (customerportalapp.MallProduct, error)
	UpdateMallProductImage(context.Context, customerportalapp.UpdateMallProductImageCommand) (customerportalapp.MallProduct, error)
	GetMallPage(context.Context, string) (customerportalapp.MallPage, error)
	CreateMallOrder(context.Context, string, customerportalapp.CreateMallOrderCommand) (customerportalapp.FulfillmentOrder, error)
	CreateDirectShipBatch(context.Context, string, customerportalapp.CreateDirectShipBatchCommand) (customerportalapp.DirectShipBatch, error)
	CreateProcessingRequest(context.Context, string, customerportalapp.CreateProcessingRequestCommand) (customerportalapp.ProcessingRequest, error)
	CreateFulfillmentOrder(context.Context, string, customerportalapp.CreateFulfillmentOrderCommand) (customerportalapp.FulfillmentOrder, error)
	EnsureOrderAccess(context.Context, string, int64) error
}

type Dependencies struct {
	CustomerPortal      Service
	MessageCenter       MessagePublisher
	BeanListPDFRenderer BeanListPDFRenderer
	SalesDocuments      SalesDocuments
	AssetDir            string
}

type MessagePublisher interface {
	Publish(context.Context, messagecenterapp.PublishCommand) (int64, error)
}

type BeanListPDFRenderer interface {
	Render(pdfinfra.BeanListDocument) ([]byte, error)
}

type SalesDocuments interface {
	LoadSalesOrderDocumentFile(context.Context, int64, int64, bool) (salesapp.SalesOrderDocumentFile, error)
	LoadDeliveryNoteDocumentFile(context.Context, int64, int64, bool) (salesapp.DeliveryNoteDocumentFile, error)
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	renderer := deps.BeanListPDFRenderer
	if renderer == nil {
		renderer = pdfinfra.BeanListRenderer{}
	}
	registerMiniAPI(e, deps.CustomerPortal, deps.MessageCenter, renderer, deps.SalesDocuments)
	registerAdminAPI(e, deps.CustomerPortal, deps.AssetDir)
}

type StaticIdentityProvider struct {
	OpenID  string
	UnionID string
}

func (p StaticIdentityProvider) Resolve(ctx context.Context, code string) (customerportalapp.MiniIdentity, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("code required")
	}
	openID := strings.TrimSpace(p.OpenID)
	if openID == "" {
		openID = "dev-openid-" + code
	}
	return customerportalapp.MiniIdentity{OpenID: openID, UnionID: strings.TrimSpace(p.UnionID)}, nil
}

func (p StaticIdentityProvider) ResolvePhoneNumber(ctx context.Context, code string) (customerportalapp.MiniPhoneNumber, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return customerportalapp.MiniPhoneNumber{}, fmt.Errorf("phone_code required")
	}
	return customerportalapp.MiniPhoneNumber{PhoneNumber: code, PurePhoneNumber: code, CountryCode: "86"}, nil
}

type DisabledIdentityProvider struct{}

func (DisabledIdentityProvider) Resolve(ctx context.Context, code string) (customerportalapp.MiniIdentity, error) {
	return customerportalapp.MiniIdentity{}, customerportalapp.ErrMiniLoginDisabled
}

const defaultWechatCode2SessionEndpoint = "https://api.weixin.qq.com/sns/jscode2session"
const defaultWechatAccessTokenEndpoint = "https://api.weixin.qq.com/cgi-bin/token"
const defaultWechatPhoneNumberEndpoint = "https://api.weixin.qq.com/wxa/business/getuserphonenumber"

type WechatIdentityProvider struct {
	AppID               string
	AppSecret           string
	Endpoint            string
	AccessTokenEndpoint string
	PhoneNumberEndpoint string
	Client              *http.Client
}

type wechatCode2SessionResponse struct {
	OpenID  string `json:"openid"`
	UnionID string `json:"unionid"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type wechatAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type wechatPhoneNumberInfo struct {
	PhoneNumber     string `json:"phoneNumber"`
	PurePhoneNumber string `json:"purePhoneNumber"`
	CountryCode     string `json:"countryCode"`
}

type wechatPhoneNumberResponse struct {
	PhoneInfo wechatPhoneNumberInfo `json:"phone_info"`
	ErrCode   int                   `json:"errcode"`
	ErrMsg    string                `json:"errmsg"`
}

func (p WechatIdentityProvider) Resolve(ctx context.Context, code string) (customerportalapp.MiniIdentity, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("code required")
	}
	appID := strings.TrimSpace(p.AppID)
	appSecret := strings.TrimSpace(p.AppSecret)
	if appID == "" || appSecret == "" {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("wechat mini app credentials required")
	}
	endpoint := strings.TrimSpace(p.Endpoint)
	if endpoint == "" {
		endpoint = defaultWechatCode2SessionEndpoint
	}
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return customerportalapp.MiniIdentity{}, err
	}
	q := reqURL.Query()
	q.Set("appid", appID)
	q.Set("secret", appSecret)
	q.Set("js_code", code)
	q.Set("grant_type", "authorization_code")
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return customerportalapp.MiniIdentity{}, err
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return customerportalapp.MiniIdentity{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("wechat code2session http status %d", resp.StatusCode)
	}
	var payload wechatCode2SessionResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return customerportalapp.MiniIdentity{}, err
	}
	if payload.ErrCode != 0 {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("wechat code2session failed: %d %s", payload.ErrCode, payload.ErrMsg)
	}
	openID := strings.TrimSpace(payload.OpenID)
	if openID == "" {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("openid required")
	}
	return customerportalapp.MiniIdentity{OpenID: openID, UnionID: strings.TrimSpace(payload.UnionID)}, nil
}

func (p WechatIdentityProvider) ResolvePhoneNumber(ctx context.Context, code string) (customerportalapp.MiniPhoneNumber, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return customerportalapp.MiniPhoneNumber{}, fmt.Errorf("phone_code required")
	}
	accessToken, err := p.accessToken(ctx)
	if err != nil {
		return customerportalapp.MiniPhoneNumber{}, err
	}
	endpoint := strings.TrimSpace(p.PhoneNumberEndpoint)
	if endpoint == "" {
		endpoint = defaultWechatPhoneNumberEndpoint
	}
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return customerportalapp.MiniPhoneNumber{}, err
	}
	q := reqURL.Query()
	q.Set("access_token", accessToken)
	reqURL.RawQuery = q.Encode()

	body, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return customerportalapp.MiniPhoneNumber{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), strings.NewReader(string(body)))
	if err != nil {
		return customerportalapp.MiniPhoneNumber{}, err
	}
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return customerportalapp.MiniPhoneNumber{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return customerportalapp.MiniPhoneNumber{}, fmt.Errorf("wechat phone number http status %d", resp.StatusCode)
	}
	var payload wechatPhoneNumberResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return customerportalapp.MiniPhoneNumber{}, err
	}
	if payload.ErrCode != 0 {
		return customerportalapp.MiniPhoneNumber{}, fmt.Errorf("wechat phone number failed: %d %s", payload.ErrCode, payload.ErrMsg)
	}
	return customerportalapp.MiniPhoneNumber{
		PhoneNumber:     strings.TrimSpace(payload.PhoneInfo.PhoneNumber),
		PurePhoneNumber: strings.TrimSpace(payload.PhoneInfo.PurePhoneNumber),
		CountryCode:     strings.TrimSpace(payload.PhoneInfo.CountryCode),
	}, nil
}

func (p WechatIdentityProvider) accessToken(ctx context.Context) (string, error) {
	appID := strings.TrimSpace(p.AppID)
	appSecret := strings.TrimSpace(p.AppSecret)
	if appID == "" || appSecret == "" {
		return "", fmt.Errorf("wechat mini app credentials required")
	}
	endpoint := strings.TrimSpace(p.AccessTokenEndpoint)
	if endpoint == "" {
		endpoint = defaultWechatAccessTokenEndpoint
	}
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	q := reqURL.Query()
	q.Set("grant_type", "client_credential")
	q.Set("appid", appID)
	q.Set("secret", appSecret)
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return "", err
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("wechat access token http status %d", resp.StatusCode)
	}
	var payload wechatAccessTokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", err
	}
	if payload.ErrCode != 0 {
		return "", fmt.Errorf("wechat access token failed: %d %s", payload.ErrCode, payload.ErrMsg)
	}
	token := strings.TrimSpace(payload.AccessToken)
	if token == "" {
		return "", fmt.Errorf("wechat access token required")
	}
	return token, nil
}
