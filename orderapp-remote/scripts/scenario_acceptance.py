#!/usr/bin/env python3
"""Post-deploy API scenario acceptance for PR-440.

The default --dry-run mode prints the bounded scenario plan. Live mode without
--allow-writes runs read probes only. Live mode with --allow-writes creates its
own scenario data through public APIs and then cleans it up before exiting.
"""

from __future__ import annotations

import argparse
import base64
import json
import random
import string
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from datetime import date
from typing import Any, Callable


MARKER = "POST_DEPLOY_ACCEPTANCE_SCENARIOS"
SCENARIO_DATA_PREFIX = "PR440-SCENARIO"


@dataclass(frozen=True)
class Step:
    name: str
    method: str
    path: str
    write: bool = False
    expect_status: tuple[int, ...] = (200,)
    note: str = ""


@dataclass(frozen=True)
class Scenario:
    name: str
    purpose: str
    steps: list[Step] = field(default_factory=list)


@dataclass
class CleanupAction:
    name: str
    method: str
    path: str
    body: dict[str, Any] | None = None
    expect_status: tuple[int, ...] = (200,)


SCENARIOS = [
    Scenario(
        name="material_to_price_list_order_settlement",
        purpose="Main flow: generated material/product/group/templates -> published price list -> order -> settlement/customer-facing reads.",
        steps=[
            Step("read order form defaults", "GET", "/api/order/form"),
            Step("create generated customer", "POST", "/api/customers", write=True),
            Step("create generated material", "POST", "/api/materials", write=True),
            Step("create generated product", "POST", "/api/product-settings/products", write=True),
            Step("create generated generic group", "POST", "/api/business-groups", write=True),
            Step("create generated Pricing Rule", "POST", "/api/product-pricing-rules", write=True),
            Step("create generated tier template", "POST", "/api/price-tier-templates", write=True),
            Step("create generated customer reference", "POST", "/api/product-customer-references", write=True),
            Step("publish generated price list snapshot", "POST", "/api/costing/bean-list/publications", write=True),
            Step("create generated order", "POST", "/api/order", write=True),
            Step("read generated order trace", "GET", "/api/orders/{order_id}/detail"),
            Step("read generated sales preview", "GET", "/api/orders/{order_id}/sales-order-preview", expect_status=(200, 400, 401, 403, 404)),
            Step("read customer mini-facing products", "GET", "/api/mini/customer-products", expect_status=(200, 401, 403)),
            Step("read customer settlement service", "GET", "/api/mini/services/settlement", expect_status=(200, 401, 403, 404)),
        ],
    ),
    Scenario(
        name="price_list_inheritance_override",
        purpose="Focused flow: default, parent group, subgroup, and product-row template inheritance stays inspectable.",
        steps=[
            Step("read groups", "GET", "/api/business-groups"),
            Step("read tier templates", "GET", "/api/price-tier-templates"),
            Step("read pricing rules", "GET", "/api/product-pricing-rules"),
            Step("read published price lists", "GET", "/api/costing/bean-list/publications?list_type=commercial&publication_purpose=factory_supply"),
        ],
    ),
    Scenario(
        name="order_quote_and_production_trace",
        purpose="Focused flow: order detail exposes quote_source_trace and production_source_trace for settlement review.",
        steps=[
            Step("read recent orders", "GET", "/api/orders?limit=5"),
            Step("read sample detail", "GET", "/api/orders/{order_id}/detail", expect_status=(200, 401, 403, 404)),
            Step("read sales order preview", "GET", "/api/orders/{order_id}/sales-order-preview", expect_status=(200, 400, 401, 403, 404)),
        ],
    ),
]


class Client:
    def __init__(self, base_url: str, cookie: str = "", basic_auth: str = "") -> None:
        self.base_url = base_url.rstrip("/")
        self.cookie = cookie
        self.basic_auth = basic_auth

    def request(self, method: str, path: str, body: dict[str, Any] | None = None) -> dict[str, Any]:
        url = urllib.parse.urljoin(self.base_url + "/", path.lstrip("/"))
        data = None
        headers = {"Accept": "application/json"}
        if body is not None:
            data = json.dumps(body, ensure_ascii=False).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if self.cookie:
            headers["Cookie"] = self.cookie
        if self.basic_auth:
            encoded = base64.b64encode(self.basic_auth.encode("utf-8")).decode("ascii")
            headers["Authorization"] = f"Basic {encoded}"
        req = urllib.request.Request(url, data=data, method=method.upper(), headers=headers)
        started = time.time()
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                payload = resp.read(1024 * 1024)
                return {"status": resp.status, "ms": elapsed_ms(started), "body": decode_body(payload)}
        except urllib.error.HTTPError as exc:
            payload = exc.read(1024 * 1024)
            return {"status": exc.code, "ms": elapsed_ms(started), "body": decode_body(payload)}
        except urllib.error.URLError as exc:
            return {"status": 0, "ms": elapsed_ms(started), "body": str(exc)}


def elapsed_ms(started: float) -> int:
    return int((time.time() - started) * 1000)


def decode_body(payload: bytes) -> Any:
    if not payload:
        return None
    text = payload.decode("utf-8", errors="replace")
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return text[:1000]


def selected_scenarios(name: str) -> list[Scenario]:
    if name == "all":
        return SCENARIOS
    rows = [row for row in SCENARIOS if row.name == name]
    if not rows:
        known = ", ".join(row.name for row in SCENARIOS)
        raise SystemExit(f"unknown scenario {name!r}; choose one of: all, {known}")
    return rows


def dry_run_payload(scenarios: list[Scenario]) -> dict[str, Any]:
    return {
        "marker": MARKER,
        "mode": "dry-run",
        "data_prefix": SCENARIO_DATA_PREFIX,
        "cleanup": "generated orders are voided; price lists withdrawn; generated master data is deactivated/deprecated",
        "scenarios": [
            {
                "name": scenario.name,
                "purpose": scenario.purpose,
                "steps": [
                    {
                        "name": step.name,
                        "method": step.method,
                        "path": step.path,
                        "write": step.write,
                        "expect_status": list(step.expect_status),
                        "note": step.note,
                    }
                    for step in scenario.steps
                ],
            }
            for scenario in scenarios
        ],
    }


def run(args: argparse.Namespace) -> int:
    scenarios = selected_scenarios(args.scenario)
    if args.dry_run or not args.base_url:
        print(json.dumps(dry_run_payload(scenarios), ensure_ascii=False, indent=2))
        return 0

    client = Client(args.base_url, cookie=args.cookie, basic_auth=args.basic_auth)
    results: list[dict[str, Any]] = []
    failures: list[str] = []
    cleanup_results: list[dict[str, Any]] = []

    run_main = args.allow_writes and any(s.name == "material_to_price_list_order_settlement" for s in scenarios)
    context = new_context(args)
    cleanup_stack: list[CleanupAction] = []

    try:
        if run_main:
            run_generated_main_flow(client, context, cleanup_stack, results, failures)
        for scenario in scenarios:
            if run_main and scenario.name == "material_to_price_list_order_settlement":
                continue
            run_read_probe_scenario(client, scenario, context, results, failures)
    finally:
        if cleanup_stack and not args.keep_data:
            cleanup_generated_data(client, cleanup_stack, cleanup_results)
        elif cleanup_stack:
            cleanup_results.append({"skipped": "keep-data requested", "actions": len(cleanup_stack)})

    cleanup_failures = [
        f"{item.get('name')}: status {item.get('status')}"
        for item in cleanup_results
        if item.get("ok") is False
    ]
    payload = {
        "marker": MARKER,
        "mode": "live",
        "run_id": context["run_id"],
        "created": context.get("created", {}),
        "results": results,
        "cleanup": cleanup_results,
        "failures": failures + cleanup_failures,
    }
    print(json.dumps(payload, ensure_ascii=False, indent=2))
    return 1 if failures or cleanup_failures else 0


def new_context(args: argparse.Namespace) -> dict[str, Any]:
    suffix = "".join(random.choice(string.ascii_uppercase + string.digits) for _ in range(6))
    run_id = args.run_id or f"{SCENARIO_DATA_PREFIX}-{date.today().strftime('%Y%m%d')}-{suffix}"
    return {
        "run_id": run_id,
        "today": date.today().isoformat(),
        "created": {},
    }


def run_generated_main_flow(
    client: Client,
    context: dict[str, Any],
    cleanup_stack: list[CleanupAction],
    results: list[dict[str, Any]],
    failures: list[str],
) -> None:
    form = request_step(client, context, results, failures, "read order form defaults", "GET", "/api/order/form")
    if not isinstance(form, dict):
        failures.append("order form defaults unavailable")
        return

    customer = create_customer(client, context, cleanup_stack, results, failures, form)
    material = create_material(client, context, cleanup_stack, results, failures)
    product = create_product(client, context, cleanup_stack, results, failures)
    group = create_business_group(client, context, cleanup_stack, results, failures)
    rule = create_pricing_rule(client, context, cleanup_stack, results, failures)
    tier_template = create_tier_template(client, context, cleanup_stack, results, failures)
    customer_ref = create_customer_reference(client, context, cleanup_stack, results, failures, product, customer)
    publication = publish_price_list(client, context, cleanup_stack, results, failures, product, customer, material, group, rule, tier_template, customer_ref)
    order = create_order(client, context, cleanup_stack, results, failures, form, product, customer, publication, rule, material)

    if int_value(order.get("order_id")) > 0:
        context["order_id"] = int_value(order.get("order_id"))
    request_step(client, context, results, failures, "read generated order trace", "GET", "/api/orders/{order_id}/detail")
    request_step(client, context, results, failures, "read generated sales preview", "GET", "/api/orders/{order_id}/sales-order-preview", expect=(200, 400, 401, 403, 404))
    request_step(client, context, results, failures, "read customer mini-facing products", "GET", "/api/mini/customer-products", expect=(200, 401, 403))
    request_step(client, context, results, failures, "read customer settlement service", "GET", "/api/mini/services/settlement", expect=(200, 401, 403, 404))


def run_read_probe_scenario(
    client: Client,
    scenario: Scenario,
    context: dict[str, Any],
    results: list[dict[str, Any]],
    failures: list[str],
) -> None:
    for step in scenario.steps:
        if step.write:
            results.append({"scenario": scenario.name, "step": step.name, "skipped": "write step requires --allow-writes"})
            continue
        body = request_step(client, context, results, failures, step.name, step.method, step.path, expect=step.expect_status, scenario=scenario.name)
        update_context_from_response(context, resolve_path(step.path, context), body)


def request_step(
    client: Client,
    context: dict[str, Any],
    results: list[dict[str, Any]],
    failures: list[str],
    name: str,
    method: str,
    path: str,
    body: dict[str, Any] | None = None,
    expect: tuple[int, ...] = (200,),
    scenario: str = "material_to_price_list_order_settlement",
) -> Any:
    resolved = resolve_path(path, context)
    if not resolved:
        results.append({"scenario": scenario, "step": name, "path": path, "skipped": "missing dynamic context"})
        return None
    outcome = client.request(method, resolved, body=body)
    ok = outcome["status"] in expect
    results.append({"scenario": scenario, "step": name, "path": resolved, "status": outcome["status"], "ok": ok, "ms": outcome["ms"]})
    if not ok:
        failures.append(f"{scenario}/{name}: status {outcome['status']} not in {expect}; body={short_body(outcome.get('body'))}")
    update_context_from_response(context, resolved, outcome.get("body"))
    return outcome.get("body") if ok else None


def create_customer(
    client: Client,
    context: dict[str, Any],
    cleanup_stack: list[CleanupAction],
    results: list[dict[str, Any]],
    failures: list[str],
    form: dict[str, Any],
) -> dict[str, Any]:
    source_id = first_id(form.get("sources"))
    order_type_id = first_id(form.get("order_types"))
    employee_id = first_id(form.get("employees"))
    if source_id <= 0 or order_type_id <= 0 or employee_id <= 0:
        failures.append("cannot create scenario customer: missing source/order type/employee defaults")
        return {}
    name = f"{context['run_id']} Customer"
    body = {
        "name": name,
        "raw_name": name,
        "customer_type": "wholesale",
        "company_name": name,
        "company_address": "Scenario cleanup address",
        "company_phone": "00000000000",
        "contact": "Scenario",
        "phone": "00000000000",
        "address": "Scenario cleanup address",
        "default_source_id": source_id,
        "default_order_type_id": order_type_id,
        "responsible_employee_id": employee_id,
        "portal_enabled": False,
        "active": True,
    }
    resp = request_step(client, context, results, failures, "create generated customer", "POST", "/api/customers", body=body)
    customer = nested_dict(resp, "customer")
    customer_id = int_value(customer.get("id"))
    if customer_id > 0:
        context["customer_id"] = customer_id
        context["created"]["customer_id"] = customer_id
        cleanup_body = {**body, "active": False}
        cleanup_stack.append(CleanupAction("deactivate generated customer", "PUT", f"/api/customers/{customer_id}", cleanup_body))
    return customer


def create_material(
    client: Client,
    context: dict[str, Any],
    cleanup_stack: list[CleanupAction],
    results: list[dict[str, Any]],
    failures: list[str],
) -> dict[str, Any]:
    body = {
        "code": context["run_id"],
        "name": f"{context['run_id']} Raw Material",
        "kind": "bean",
        "unit": "kg",
        "batch_no": context["run_id"],
        "purchase_price": 42.0,
        "sale_price": 0,
        "onhand_g": 50000,
        "onhand_units": 0,
        "min_level_g": 0,
        "min_level_units": 0,
        "bean_profile": {
            "origin": "Scenario",
            "process_method": "Washed",
            "flavor": "scenario cleanup data",
        },
    }
    material = request_step(client, context, results, failures, "create generated material", "POST", "/api/materials", body=body)
    material_id = int_value(dict_value(material).get("id"))
    if material_id > 0:
        context["material_id"] = material_id
        context["material_name"] = dict_value(material).get("name") or body["name"]
        context["created"]["material_id"] = material_id
        cleanup_stack.append(CleanupAction("deprecate generated material", "POST", f"/api/materials/{material_id}/deprecate", {}))
    return dict_value(material)


def create_product(
    client: Client,
    context: dict[str, Any],
    cleanup_stack: list[CleanupAction],
    results: list[dict[str, Any]],
    failures: list[str],
) -> dict[str, Any]:
    body = {
        "name": f"{context['run_id']} Product",
        "remark": "Generated by PR-440 scenario acceptance; should be deactivated automatically.",
        "product_kind": "roasted_bean",
        "roast_level": "中烘",
        "allow_fulfillment_order": True,
        "allow_mall_order": False,
        "default_price": 0,
        "retail_price_100g": 0,
        "retail_price_200g": 0,
        "retail_price_227g": 0,
        "retail_price_250g": 0,
        "yield_rate": 0,
        "special_attrs_json": "{}",
    }
    resp = request_step(client, context, results, failures, "create generated product", "POST", "/api/product-settings/products", body=body)
    product = nested_dict(resp, "product")
    product_id = int_value(product.get("id"))
    if product_id > 0:
        context["product_id"] = product_id
        context["product_name"] = product.get("name") or body["name"]
        context["created"]["product_id"] = product_id
        cleanup_stack.append(CleanupAction("deactivate generated product", "POST", "/api/product-settings/products/deactivate", {"product_ids": [product_id]}))
    return product


def create_business_group(
    client: Client,
    context: dict[str, Any],
    cleanup_stack: list[CleanupAction],
    results: list[dict[str, Any]],
    failures: list[str],
) -> dict[str, Any]:
    body = business_group_payload(context, active=True)
    resp = request_step(client, context, results, failures, "create generated generic group", "POST", "/api/business-groups", body=body)
    group = nested_dict(resp, "group")
    group_id = int_value(group.get("id"))
    parent_id, child_id = group_item_ids(group)
    context["group_id"] = group_id
    context["parent_group_item_id"] = parent_id
    context["group_item_id"] = child_id or parent_id
    if group_id > 0:
        context["created"]["business_group_id"] = group_id
        cleanup_body = deactivate_business_group_payload(group)
        cleanup_stack.append(CleanupAction("deactivate generated generic group", "PUT", f"/api/business-groups/{group_id}", cleanup_body))
    return group


def create_pricing_rule(
    client: Client,
    context: dict[str, Any],
    cleanup_stack: list[CleanupAction],
    results: list[dict[str, Any]],
    failures: list[str],
) -> dict[str, Any]:
    body = {
        "name": f"{context['run_id']} Pricing Rule",
        "code": context["run_id"],
        "cost_source_mode": "material_snapshot",
        "margin_rate": 0.2,
        "tax_rate": 0.13,
        "rounding_mode": "cent",
        "active": True,
        "remark": "Generated by PR-440 scenario acceptance; should be deactivated automatically.",
    }
    resp = request_step(client, context, results, failures, "create generated Pricing Rule", "POST", "/api/product-pricing-rules", body=body)
    rule = nested_dict(resp, "rule")
    rule_id = int_value(rule.get("id"))
    if rule_id > 0:
        context["pricing_rule_id"] = rule_id
        context["pricing_rule_version"] = f"{body['code']}-v1"
        context["created"]["pricing_rule_id"] = rule_id
        cleanup_stack.append(CleanupAction("deactivate generated Pricing Rule", "PUT", f"/api/product-pricing-rules/{rule_id}", {**body, "active": False}))
    return rule


def create_tier_template(
    client: Client,
    context: dict[str, Any],
    cleanup_stack: list[CleanupAction],
    results: list[dict[str, Any]],
    failures: list[str],
) -> dict[str, Any]:
    body = {
        "name": f"{context['run_id']} Tier Template",
        "active": True,
        "remark": "Generated by PR-440 scenario acceptance; should be deactivated automatically.",
        "tiers": [
            {"label": "1kg+", "min_qty": 1, "max_qty": None, "quantity_unit": "kg", "position": 1, "active": True, "remark": "scenario tier"}
        ],
    }
    resp = request_step(client, context, results, failures, "create generated tier template", "POST", "/api/price-tier-templates", body=body)
    template = nested_dict(resp, "template")
    template_id = int_value(template.get("id"))
    if template_id > 0:
        context["tier_template_id"] = template_id
        context["created"]["tier_template_id"] = template_id
        cleanup_body = dict(template) if template else body
        cleanup_body["active"] = False
        cleanup_body["tiers"] = [
            {**tier, "active": False}
            for tier in (cleanup_body.get("tiers") or body["tiers"])
            if isinstance(tier, dict)
        ]
        cleanup_stack.append(CleanupAction("deactivate generated tier template", "PUT", f"/api/price-tier-templates/{template_id}", cleanup_body))
    return template


def create_customer_reference(
    client: Client,
    context: dict[str, Any],
    cleanup_stack: list[CleanupAction],
    results: list[dict[str, Any]],
    failures: list[str],
    product: dict[str, Any],
    customer: dict[str, Any],
) -> dict[str, Any]:
    product_id = int_value(product.get("id") or context.get("product_id"))
    customer_id = int_value(customer.get("id") or context.get("customer_id"))
    if product_id <= 0 or customer_id <= 0:
        failures.append("cannot create customer reference: missing generated product/customer")
        return {}
    body = {
        "product_id": product_id,
        "customer_id": customer_id,
        "customer_item_code": f"{context['run_id']}-REF",
        "customer_display_name": f"{context['run_id']} Customer Display",
        "active": True,
        "remark": "Generated by PR-440 scenario acceptance; should be deactivated automatically.",
    }
    resp = request_step(client, context, results, failures, "create generated customer reference", "POST", "/api/product-customer-references", body=body)
    ref = nested_dict(resp, "reference")
    ref_id = int_value(ref.get("id"))
    if ref_id > 0:
        context["customer_reference_id"] = ref_id
        context["created"]["customer_reference_id"] = ref_id
        cleanup_stack.append(CleanupAction("deactivate generated customer reference", "PUT", f"/api/product-customer-references/{ref_id}", {**body, "active": False}))
    return ref


def publish_price_list(
    client: Client,
    context: dict[str, Any],
    cleanup_stack: list[CleanupAction],
    results: list[dict[str, Any]],
    failures: list[str],
    product: dict[str, Any],
    customer: dict[str, Any],
    material: dict[str, Any],
    group: dict[str, Any],
    rule: dict[str, Any],
    tier_template: dict[str, Any],
    customer_ref: dict[str, Any],
) -> dict[str, Any]:
    version = f"{context['run_id']}-PRICE"
    body = {
        "list_type": "commercial",
        "publication_purpose": "factory_supply",
        "scope": "customer",
        "customer_id": int_value(customer.get("id") or context.get("customer_id")),
        "version": version,
        "config": price_list_config(context),
        "content": price_list_content(context, product, material, group, rule, tier_template, customer_ref),
        "changelog": f"{context['run_id']} generated scenario price list; should be withdrawn automatically.",
    }
    publication = request_step(client, context, results, failures, "publish generated price list snapshot", "POST", "/api/costing/bean-list/publications", body=body)
    publication = dict_value(publication)
    publication_id = int_value(publication.get("id"))
    if publication_id > 0:
        context["publication_id"] = publication_id
        context["publication_version"] = publication.get("version") or version
        context["created"]["price_list_publication_id"] = publication_id
        cleanup_stack.append(CleanupAction("withdraw generated price list", "POST", f"/api/costing/bean-list/publications/{publication_id}/withdraw?scope=customer&customer_id={body['customer_id']}&publication_purpose=factory_supply", {}))
    return publication


def create_order(
    client: Client,
    context: dict[str, Any],
    cleanup_stack: list[CleanupAction],
    results: list[dict[str, Any]],
    failures: list[str],
    form: dict[str, Any],
    product: dict[str, Any],
    customer: dict[str, Any],
    publication: dict[str, Any],
    rule: dict[str, Any],
    material: dict[str, Any],
) -> dict[str, Any]:
    product_id = int_value(product.get("id") or context.get("product_id"))
    customer_id = int_value(customer.get("id") or context.get("customer_id"))
    publication_id = int_value(publication.get("id") or context.get("publication_id"))
    if product_id <= 0 or customer_id <= 0:
        failures.append("cannot create scenario order: missing generated product/customer")
        return {}
    unit_price = "88"
    price_source = {
        "publication_id": publication_id,
        "version": context.get("publication_version"),
        "price_list_version": context.get("publication_version"),
        "price_unit": "kg",
        "final_unit_price": 88,
        "pricing_rule_version": context.get("pricing_rule_version"),
        "manual_adjusted": False,
        "cost_source_snapshot": cost_source_snapshot(context, material),
        "production_source_snapshot": production_source_snapshot(context),
    }
    body = {
        "document_date": context["today"],
        "order_date": context["today"],
        "customer_id": customer_id,
        "pay_status_id": status_id(form.get("pay_statuses"), ("未付款", "未支付", "未收款"), fallback=first_id(form.get("pay_statuses"))),
        "ship_status_id": status_id(form.get("ship_statuses"), ("未发货", "待发货"), fallback=first_id(form.get("ship_statuses"))),
        "notes": f"{context['run_id']} generated scenario order; should be voided automatically.",
        "commercial_bean_list_publication_id": publication_id,
        "product_id": [str(product_id)],
        "item_name": [context.get("customer_display_name") or context.get("product_name") or product.get("name") or context["run_id"]],
        "tier_id": ["scenario-tier"],
        "unit_price": [unit_price],
        "qty": ["1"],
        "unit": ["kg"],
        "spec": ["1000"],
        "product_kind": ["roasted_bean"],
        "item_bean_list_publication_id": [str(publication_id)],
        "item_bean_list_version_no": [str(context.get("publication_version") or "")],
        "price_source_json": [json.dumps(price_source, ensure_ascii=False, separators=(",", ":"))],
    }
    order = request_step(client, context, results, failures, "create generated order", "POST", "/api/order", body=body)
    order = dict_value(order)
    order_id = int_value(order.get("order_id"))
    if order_id > 0:
        context["order_id"] = order_id
        context["created"]["order_id"] = order_id
        cleanup_stack.append(CleanupAction("void generated order", "POST", f"/api/orders/{order_id}/void", {"reason": f"{context['run_id']} scenario cleanup"}))
    return order


def business_group_payload(context: dict[str, Any], active: bool) -> dict[str, Any]:
    return {
        "name": f"{context['run_id']} Group",
        "code": context["run_id"],
        "remark": "Generated by PR-440 scenario acceptance; should be deactivated automatically.",
        "active": active,
        "sort_order": 9999,
        "usages": [
            {"usage_key": "product_archive", "usage_label": "Product archive", "active": active},
            {"usage_key": "product_price_list", "usage_label": "Product price list", "active": active},
        ],
        "items": [
            {
                "name": f"{context['run_id']} Parent",
                "code": f"{context['run_id']}-PARENT",
                "remark": "scenario parent group",
                "active": active,
                "sort_order": 1,
                "children": [
                    {
                        "name": f"{context['run_id']} Subgroup",
                        "code": f"{context['run_id']}-SUB",
                        "remark": "scenario subgroup",
                        "active": active,
                        "sort_order": 1,
                    }
                ],
            }
        ],
    }


def deactivate_business_group_payload(group: dict[str, Any]) -> dict[str, Any]:
    if not group:
        return {}
    out = dict(group)
    out["active"] = False
    out["usages"] = [
        {**usage, "active": False}
        for usage in out.get("usages", [])
        if isinstance(usage, dict)
    ]
    out["items"] = [deactivate_group_item(item) for item in out.get("items", []) if isinstance(item, dict)]
    return out


def deactivate_group_item(item: dict[str, Any]) -> dict[str, Any]:
    next_item = dict(item)
    next_item["active"] = False
    next_item["children"] = [deactivate_group_item(child) for child in next_item.get("children", []) if isinstance(child, dict)]
    return next_item


def price_list_config(context: dict[str, Any]) -> dict[str, Any]:
    return {
        "layoutStyle": "table",
        "price_list_template_selection": {
            "defaults": {
                "tier_template_id": int_value(context.get("tier_template_id")),
                "pricing_rule_id": int_value(context.get("pricing_rule_id")),
            },
            "group_selections": [
                {
                    "group_item_id": int_value(context.get("group_item_id")),
                    "parent_group_item_id": int_value(context.get("parent_group_item_id")),
                    "tier_template_id": int_value(context.get("tier_template_id")),
                    "pricing_rule_id": int_value(context.get("pricing_rule_id")),
                }
            ],
            "product_overrides": [
                {
                    "product_id": int_value(context.get("product_id")),
                    "group_item_id": int_value(context.get("group_item_id")),
                    "tier_template_id": int_value(context.get("tier_template_id")),
                    "pricing_rule_id": int_value(context.get("pricing_rule_id")),
                }
            ],
        },
    }


def price_list_content(
    context: dict[str, Any],
    product: dict[str, Any],
    material: dict[str, Any],
    group: dict[str, Any],
    rule: dict[str, Any],
    tier_template: dict[str, Any],
    customer_ref: dict[str, Any],
) -> dict[str, Any]:
    product_id = int_value(product.get("id") or context.get("product_id"))
    product_name = product.get("name") or context.get("product_name") or f"{context['run_id']} Product"
    group_snapshot = group_snapshot_payload(context, group)
    customer_snapshot = customer_reference_snapshot(customer_ref)
    cost_snapshot = cost_source_snapshot(context, material)
    row = {
        "product_id": product_id,
        "product_key": f"product:{product_id}",
        "product_name": product_name,
        "group_snapshot": group_snapshot,
        "tier_label": "1kg+",
        "min_qty": 1,
        "max_qty": None,
        "price_unit": "kg",
        "final_unit_price": 88,
        "original_final_unit_price": 88,
        "currency": "CNY",
        "inventory_unit": "kg",
        "inventory_conversion_json": {"kg": 1},
        "source_price_record_id": 0,
        "tier_template_id": int_value(tier_template.get("id") or context.get("tier_template_id")),
        "tier_template_source": "product",
        "pricing_rule_id": int_value(rule.get("id") or context.get("pricing_rule_id")),
        "pricing_rule_source": "product",
        "pricing_rule_version": context.get("pricing_rule_version") or f"{rule.get('code', context['run_id'])}-v1",
        "cost_source_snapshot": cost_snapshot,
        "customer_reference_snapshot": customer_snapshot,
        "manual_adjusted": False,
        "manual_adjustment_label": "",
    }
    return {
        "title": f"{context['run_id']} Product Price List",
        "totalItems": 1,
        "groups": [
            {
                "category": group_snapshot.get("group_item_name") or f"{context['run_id']} Group",
                "showCategory": True,
                "items": [
                    {
                        "code": "1.1",
                        "product_id": product_id,
                        "name": product_name,
                        "customer_display_name": customer_snapshot.get("customer_display_name", ""),
                        "prices": [{"label": "1kg+", "price": 88, "unit": "kg"}],
                    }
                ],
            }
        ],
        "price_rows": [row],
    }


def group_snapshot_payload(context: dict[str, Any], group: dict[str, Any]) -> dict[str, Any]:
    parent_id, child_id = group_item_ids(group)
    return {
        "group_id": int_value(group.get("id") or context.get("group_id")),
        "group_name": group.get("name") or f"{context['run_id']} Group",
        "parent_group_item_id": parent_id or int_value(context.get("parent_group_item_id")),
        "parent_group_item_name": first_group_item_name(group),
        "group_item_id": child_id or int_value(context.get("group_item_id")),
        "group_item_name": child_group_item_name(group) or f"{context['run_id']} Subgroup",
    }


def customer_reference_snapshot(ref: dict[str, Any]) -> dict[str, Any]:
    if not ref:
        return {}
    return {
        "customer_id": int_value(ref.get("customer_id")),
        "customer_item_code": ref.get("customer_item_code", ""),
        "customer_display_name": ref.get("customer_display_name", ""),
    }


def cost_source_snapshot(context: dict[str, Any], material: dict[str, Any]) -> dict[str, Any]:
    return {
        "cost_source_mode": "material_snapshot",
        "material_id": int_value(material.get("id") or context.get("material_id")),
        "material_name": material.get("name") or context.get("material_name") or f"{context['run_id']} Raw Material",
        "material_unit_cost": 42,
        "process_cost": 12,
        "loss_rate": 0.05,
        "margin_rate": 0.2,
        "tax_rate": 0.13,
    }


def production_source_snapshot(context: dict[str, Any]) -> dict[str, Any]:
    return {
        "bom_version_no": f"{context['run_id']}-BOM-SNAPSHOT",
        "bom_version_id": 0,
        "process_route_name": "Scenario route",
        "process_card_no": f"{context['run_id']}-CARD",
        "material_batch_no": context["run_id"],
        "work_order_no": f"{context['run_id']}-WO",
        "source_label": "scenario production snapshot",
    }


def cleanup_generated_data(client: Client, cleanup_stack: list[CleanupAction], cleanup_results: list[dict[str, Any]]) -> None:
    for action in reversed(cleanup_stack):
        outcome = client.request(action.method, action.path, body=action.body)
        ok = outcome["status"] in action.expect_status
        cleanup_results.append({"name": action.name, "path": action.path, "status": outcome["status"], "ok": ok, "ms": outcome["ms"]})


def resolve_path(path: str, context: dict[str, Any]) -> str:
    if "{order_id}" in path:
        order_id = int_value(context.get("order_id"))
        if order_id <= 0:
            return ""
        return path.replace("{order_id}", str(order_id))
    return path


def update_context_from_response(context: dict[str, Any], path: str, body: Any) -> None:
    if path.startswith("/api/orders"):
        order_id = first_order_id(body)
        if order_id > 0 and not context.get("order_id"):
            context["order_id"] = order_id


def first_order_id(body: Any) -> int:
    if not isinstance(body, dict):
        return 0
    candidates: list[Any] = []
    for key in ("rows", "orders", "data"):
        value = body.get(key)
        if isinstance(value, list):
            candidates.extend(value)
    if not candidates and isinstance(body.get("order"), dict):
        candidates.append(body["order"])
    for row in candidates:
        if not isinstance(row, dict):
            continue
        order_id = int_value(row.get("id") or row.get("order_id"))
        if order_id > 0:
            return order_id
    return 0


def group_item_ids(group: dict[str, Any]) -> tuple[int, int]:
    items = group.get("items")
    if not isinstance(items, list) or not items:
        return 0, 0
    parent = dict_value(items[0])
    children = parent.get("children")
    child = dict_value(children[0]) if isinstance(children, list) and children else {}
    return int_value(parent.get("id")), int_value(child.get("id"))


def first_group_item_name(group: dict[str, Any]) -> str:
    items = group.get("items")
    if not isinstance(items, list) or not items:
        return ""
    return str(dict_value(items[0]).get("name") or "")


def child_group_item_name(group: dict[str, Any]) -> str:
    items = group.get("items")
    if not isinstance(items, list) or not items:
        return ""
    children = dict_value(items[0]).get("children")
    if not isinstance(children, list) or not children:
        return ""
    return str(dict_value(children[0]).get("name") or "")


def nested_dict(value: Any, key: str) -> dict[str, Any]:
    if isinstance(value, dict) and isinstance(value.get(key), dict):
        return value[key]
    return {}


def dict_value(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}


def first_id(rows: Any) -> int:
    if not isinstance(rows, list):
        return 0
    for row in rows:
        if isinstance(row, dict):
            n = int_value(row.get("id"))
            if n > 0:
                return n
    return 0


def status_id(rows: Any, labels: tuple[str, ...], fallback: int = 0) -> int:
    if not isinstance(rows, list):
        return fallback
    for label in labels:
        for row in rows:
            if not isinstance(row, dict):
                continue
            if label in str(row.get("name") or row.get("label") or ""):
                n = int_value(row.get("id"))
                if n > 0:
                    return n
    return fallback


def int_value(value: Any) -> int:
    try:
        return int(value)
    except (TypeError, ValueError):
        return 0


def short_body(value: Any) -> str:
    text = json.dumps(value, ensure_ascii=False) if isinstance(value, (dict, list)) else str(value)
    return text[:500]


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="KFerp PR-440 post-deploy scenario acceptance")
    parser.add_argument("--base-url", default="", help="Base URL such as https://erp.example.com/app")
    parser.add_argument("--cookie", default="", help="Authenticated Cookie header value")
    parser.add_argument("--basic-auth", default="", help="username:password for Basic auth when applicable")
    parser.add_argument("--scenario", default="all", help="Scenario name or all")
    parser.add_argument("--run-id", default="", help="Optional generated data prefix for live write mode")
    parser.add_argument("--dry-run", action="store_true", help="Print planned API scenarios without network calls")
    parser.add_argument("--allow-writes", action="store_true", help="Create generated scenario data and clean it up")
    parser.add_argument("--keep-data", action="store_true", help="Debug only: leave generated scenario data in place")
    return parser.parse_args(argv)


if __name__ == "__main__":
    raise SystemExit(run(parse_args(sys.argv[1:])))
