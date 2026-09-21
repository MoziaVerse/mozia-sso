#!/usr/bin/env python3
"""Smoke-test a running local Casdoor against a DISPOSABLE phone_auth_test DB.

Seeds challenges and exercises a loopback-only SMS stub instead of sending SMS. Requires psql and a bootstrapped server.
No production credentials, phone numbers, or account data are used.
"""
import base64
import concurrent.futures
from datetime import datetime, timezone
import hashlib
import http.cookiejar
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import threading
import json
import os
import secrets
import subprocess
import urllib.error
import urllib.parse
import urllib.request

BASE = os.environ.get("PHONE_AUTH_TEST_BASE_URL", "http://127.0.0.1:21878")
DSN = os.environ["PHONE_AUTH_TEST_POSTGRES"]
if urllib.parse.urlparse(BASE).hostname not in ("127.0.0.1", "localhost"):
    raise SystemExit("Only a local test server is allowed")


def sql(statement):
    return subprocess.check_output(
        ["psql", DSN, "-X", "-v", "ON_ERROR_STOP=1", "-Atc", statement], text=True
    ).strip()


assert sql("SELECT current_database()") == "phone_auth_test", "Disposable DB required"
fixture = "phone-test-" + secrets.token_hex(5)
client_id = fixture + "-client"
client_secret = secrets.token_urlsafe(32)
callback = "http://127.0.0.1:21879/callback"


def quoted(value):
    return "'" + value.replace("'", "''") + "'"


def copy_fixture(table, source, changes):
    sql(
        f"INSERT INTO {table} SELECT (jsonb_populate_record(NULL::{table}, "
        f"to_jsonb(t)||{quoted(json.dumps(changes))}::jsonb)).* "
        f"FROM {table} t WHERE name={quoted(source)}"
    )


copy_fixture("organization", "built-in", {
    "name": fixture, "display_name": "Disposable Phone Test", "default_avatar": "", "init_score": 0,
})
copy_fixture("application", "app-built-in", {
    "name": fixture, "organization": fixture, "enable_phone_signin_signup": True,
    "enable_sign_up": True, "enable_signin_session": True, "enable_auto_signin": True,
    "client_id": client_id, "client_secret": client_secret, "cert": "cert-built-in",
    "redirect_uris": json.dumps([callback]),
    "signin_methods": json.dumps([{"name": "Verification code", "rule": "Phone only"}, {"name": "Password", "rule": "All"}]),
    "signup_items": json.dumps([
        {"name": "ID", "visible": False, "required": True, "rule": "Random"},
        {"name": "Username", "visible": True, "required": True},
        {"name": "Password", "visible": True, "required": True},
        {"name": "Phone", "visible": True, "required": True, "rule": "Normal"},
    ]),
    "grant_types": json.dumps(["authorization_code", "password", "refresh_token"]),
    "token_format": "JWT", "expire_in_hours": 1,
})


def browser():
    jar = http.cookiejar.CookieJar()
    return urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar)), jar


def call(client, path, body, form=False):
    data = urllib.parse.urlencode(body).encode() if form else json.dumps(body).encode()
    request = urllib.request.Request(BASE + path, data=data, headers={
        "Content-Type": "application/x-www-form-urlencoded" if form else "application/json",
    })
    try:
        response = client.open(request, timeout=30)
    except urllib.error.HTTPError as error:
        response = error
    return response.status, json.load(response)


def seed(phone, code):
    name = secrets.token_hex(12)
    sql(f"INSERT INTO verification_record (owner,name,created_time,receiver,code,time,is_used,failed_attempts,type,\"user\",provider) VALUES ({quoted(fixture)},{quoted(name)},{quoted(datetime.now(timezone.utc).isoformat())},{quoted('+86' + phone)},{quoted(code)},extract(epoch from now())::bigint,false,0,'SMS','','')")
    return name


verifier = secrets.token_urlsafe(48)
params = {
    "clientId": client_id, "responseType": "code", "redirectUri": callback,
    "scope": "openid profile email phone", "state": "test-state", "nonce": "test-nonce",
    "code_challenge_method": "S256",
    "code_challenge": base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).decode().rstrip("="),
}
login_path = "/api/login?" + urllib.parse.urlencode(params)


def body(phone, code, **extra):
    return {
        "type": "code", "application": fixture, "organization": fixture,
        "username": phone, "countryCode": "CN", "signinMethod": "Verification code",
        "code": code, "phoneSigninSignup": True, "agreement": True, "autoSignin": True, **extra,
    }


def authenticate(phone, code, **extra):
    client, jar = browser()
    _, result = call(client, login_path, body(phone, code, **extra))
    assert result.get("status") == "ok", result.get("msg")
    _, tokens = call(client, "/api/login/oauth/access_token", {
        "grant_type": "authorization_code", "client_id": client_id,
        "client_secret": client_secret, "code": result["data"],
        "redirect_uri": callback, "code_verifier": verifier,
    }, True)
    assert tokens.get("access_token") and tokens.get("id_token"), "Token exchange failed"
    part = tokens["id_token"].split(".")[1]
    claims = json.loads(base64.urlsafe_b64decode(part + "=" * (-len(part) % 4)))
    assert claims["nonce"] == params["nonce"]
    assert any(cookie.name == "casdoor_session_id" for cookie in jar)
    return client, jar, claims["sub"]


seed("13800138000", "123456")
client, jar, subject = authenticate("13800138000", "123456")
assert call(client, login_path, body("13800138000", "123456"))[1]["status"] == "error"
seed("13800138000", "234567")
client, jar, old_subject = authenticate("13800138000", "234567")
assert old_subject == subject
assert sql(f"SELECT count(*) FROM \"user\" WHERE owner={quoted(fixture)} AND phone='13800138000'") == "1"
print("PASS new/old phone: stable sub, PKCE, nonce, Session and replay rejection")

challenge = seed("13900139000", "345678")
assert call(client, login_path, body("13900139000", "345678", agreement=False))[1]["status"] == "error"
assert sql(f"SELECT is_used FROM verification_record WHERE owner={quoted(fixture)} AND name={quoted(challenge)}") == "f"
sql(f"UPDATE application SET enable_sign_up=false WHERE name={quoted(fixture)}")
assert call(client, login_path, body("13900139000", "345678"))[1]["status"] == "error"
assert sql(f"SELECT count(*) FROM \"user\" WHERE owner={quoted(fixture)} AND phone='13900139000'") == "0"
sql(f"UPDATE application SET enable_sign_up=true WHERE name={quoted(fixture)}")
assert call(client, login_path, body("13900139000", "345678", organization="built-in"))[1]["status"] == "error"
print("PASS agreement, disabled signup and organization boundaries")

seed("13700137000", "456789")
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
    results = list(pool.map(lambda _: call(browser()[0], login_path, body("13700137000", "456789"))[1]["status"], range(2)))
assert results.count("ok") == 1, results
assert sql(f"SELECT count(*) FROM \"user\" WHERE owner={quoted(fixture)} AND phone='13700137000'") == "1"
print("PASS concurrent signup creates exactly one account")


# The old Matrix signup contract remains usable during rollout.
seed("13600136000", "567890")
legacy_name = "legacy_" + secrets.token_hex(5)
legacy_password = secrets.token_urlsafe(24) + "Mz9@!"
_, legacy = call(browser()[0], "/api/signup", {
    "organization": fixture, "application": fixture, "username": legacy_name,
    "name": legacy_name, "password": legacy_password, "phone": "13600136000",
    "phoneCode": "567890", "countryCode": "CN", "autoSignin": False,
})
assert legacy.get("status") == "ok", legacy.get("msg")
_, password_login = call(browser()[0], login_path, {
    "type": "code", "organization": fixture, "application": fixture,
    "username": legacy_name, "password": legacy_password, "signinMethod": "Password",
})
assert password_login.get("status") == "ok", password_login.get("msg")
print("PASS legacy signup and password login remain compatible")

# Reject invalid OAuth context before consuming a code or creating an identity.
challenge = seed("13500135000", "678901")
for overrides in [{"redirectUri": "https://invalid.example/callback"}, {"clientId": "missing-client"}]:
    invalid_path = "/api/login?" + urllib.parse.urlencode({**params, **overrides})
    assert call(browser()[0], invalid_path, body("13500135000", "678901"))[1]["status"] == "error"
    assert sql(f"SELECT is_used FROM verification_record WHERE owner={quoted(fixture)} AND name={quoted(challenge)}") == "f"
assert sql(f"SELECT count(*) FROM \"user\" WHERE owner={quoted(fixture)} AND phone='13500135000'") == "0"
print("PASS invalid OAuth application and callback rejected before signup")

sql(f"UPDATE \"user\" SET is_forbidden=true WHERE owner={quoted(fixture)} AND phone='13800138000'")
seed("13800138000", "789012")
assert call(browser()[0], login_path, body("13800138000", "789012"))[1]["status"] == "error"
sql(f"UPDATE \"user\" SET is_forbidden=false WHERE owner={quoted(fixture)} AND phone='13800138000'")
sql(f"UPDATE application SET enable_sign_up=false WHERE name={quoted(fixture)}")
seed("13800138000", "890123")
assert authenticate("13800138000", "890123", agreement=False)[2] == subject
sql(f"UPDATE application SET enable_sign_up=true, enable_phone_signin_signup=false WHERE name={quoted(fixture)}")
seed("13800138000", "901234")
assert call(browser()[0], login_path, body("13800138000", "901234", phoneSigninSignup=False))[1]["status"] == "ok"
seed("13400134000", "012345")
assert call(browser()[0], login_path, body("13400134000", "012345"))[1]["status"] == "error"
assert sql(f"SELECT count(*) FROM \"user\" WHERE owner={quoted(fixture)} AND phone='13400134000'") == "0"
sql(f"UPDATE application SET enable_phone_signin_signup=true WHERE name={quoted(fixture)}")
print("PASS forbidden user, old-user login with signup disabled and switch-off compatibility")

class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        return None


sso = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar), NoRedirect())
try:
    response = sso.open(BASE + "/login/oauth/authorize?" + urllib.parse.urlencode({
        "client_id": client_id, "response_type": "code", "redirect_uri": callback,
        "scope": params["scope"], "state": "sso-state",
    }), timeout=30)
except urllib.error.HTTPError as error:
    response = error
assert response.status == 302
redirect = urllib.parse.urlparse(response.headers["Location"])
assert urllib.parse.parse_qs(redirect.query)["state"] == ["sso-state"]
print("PASS browser Session continues to subsequent OAuth authorization")


# Exercise the real send-code endpoint with a loopback-only SMS provider.
# No request leaves the local machine and the generated code is never printed.
sms_messages = []


class SmsStub(BaseHTTPRequestHandler):
    def do_POST(self):
        sms_messages.append(json.loads(self.rfile.read(int(self.headers["Content-Length"]))))
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"{}")

    def log_message(self, *args):
        pass


sms_stub = ThreadingHTTPServer(("127.0.0.1", 0), SmsStub)
threading.Thread(target=sms_stub.serve_forever, daemon=True).start()
provider = fixture + "-sms"
try:
    sql(f"INSERT INTO provider (owner,name,category,type,endpoint,method,title,template_code,issuer_url) VALUES ('admin',{quoted(provider)},'SMS','Custom HTTP SMS',{quoted('http://127.0.0.1:' + str(sms_stub.server_port))},'POST','code','%s','application/json')")
    sql(f"UPDATE application SET providers={quoted(json.dumps([{'name': provider, 'rule': 'All'}]))}, code_resend_timeout=-1 WHERE name={quoted(fixture)}")

    def send(phone):
        return call(browser()[0], "/api/send-verification-code", {
            "applicationId": "admin/" + fixture, "type": "phone", "method": "login",
            "dest": phone, "countryCode": "CN", "captchaType": "none", "captchaToken": "",
        }, True)

    status, result = send("13300133000")
    assert result.get("status") == "ok", result.get("msg")
    assert len(sms_messages) == 1
    status, result = send("13300133000")
    assert status == 429 and result["data"]["retryAfterSeconds"] > 0, result
    assert len(sms_messages) == 1, "Cooldown request reached SMS provider"
    authenticate("13300133000", sms_messages[-1]["code"])
    sql(f"UPDATE application SET enable_sign_up=false WHERE name={quoted(fixture)}")
    assert send("13200132000")[1]["status"] == "error"
    assert len(sms_messages) == 1, "Disabled signup reached SMS provider"
    sql(f"UPDATE application SET enable_sign_up=true WHERE name={quoted(fixture)}")
    print("PASS actual send-code endpoint: unknown phone, loopback SMS, cooldown and disabled signup")
finally:
    sms_stub.shutdown()
    sms_stub.server_close()

# Embedded login: BFF authentication -> one-use browser POST -> OIDC SSO.
return_origin = "http://127.0.0.1:21900"
return_uri = return_origin + "/login?sso_return=12345678-1234-1234-1234-123456789abc"
sql(f"UPDATE application SET embedded_signin_origins={quoted(json.dumps([return_origin]))} WHERE name={quoted(fixture)}")
embedded_header = "Basic " + base64.b64encode((client_id + ":" + client_secret).encode()).decode()


def embedded(phone, code, secret=embedded_header, uri=return_uri):
    bff, cookies = browser()
    request = urllib.request.Request(BASE + "/api/login", data=json.dumps(body(phone,code,type="login",browserReturnUri=uri)).encode(), headers={"Content-Type":"application/json", "X-Casdoor-Embedded-Client":secret})
    with bff.open(request) as response:
        result = json.load(response)
    return result, bff


challenge = seed("13100131000", "123456")
for secret, uri in [("Basic invalid", return_uri), (embedded_header, "https://evil.test/login")]:
    assert embedded("13100131000", "123456", secret, uri)[0]["status"] == "error"
    assert sql(f"SELECT is_used FROM verification_record WHERE name={quoted(challenge)} AND owner={quoted(fixture)}") == "f"
result, bff = embedded("13100131000", "123456")
assert result["status"] == "ok", result.get("msg")
assert result["data2"]["newUser"] is True
assert json.load(bff.open(BASE + "/api/userinfo"))["sub"]
ticket = result["data2"]["browserTicket"]
jar = http.cookiejar.CookieJar()
consumer = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar), NoRedirect())


def post_ticket(ticket, origin):
    headers = {"Content-Type":"application/x-www-form-urlencoded"}
    if origin is not None: headers["Origin"] = origin
    req = urllib.request.Request(BASE + "/api/browser-signin", data=urllib.parse.urlencode({"ticket":ticket}).encode(),headers=headers)
    try: return consumer.open(req)
    except urllib.error.HTTPError as error: return error


for origin in [None,"null","https://evil.test"]:
    assert post_ticket(ticket, origin).status == 400
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
    statuses = list(pool.map(lambda _: post_ticket(ticket, return_origin).status,range(2)))
assert statuses.count(303) == 1 and statuses.count(400) == 1, statuses
assert post_ticket(ticket,return_origin).status == 400
assert json.load(consumer.open(BASE + "/api/userinfo"))["sub"] == json.load(bff.open(BASE + "/api/userinfo"))["sub"]
try:
    r = consumer.open(BASE + "/login/oauth/authorize?" + urllib.parse.urlencode({"client_id":client_id,"response_type":"code","redirect_uri":callback,"scope":"openid profile","state":"embedded-sso"}))
except urllib.error.HTTPError as error: r = error
assert r.status == 302 and "state=embedded-sso" in r.headers["Location"]
seed("13100131000", "234567")
result,_ = embedded("13100131000","234567")
assert result["data2"]["newUser"] is False
sql(f"UPDATE application SET disable_signin=true WHERE name={quoted(fixture)}")
assert post_ticket(result["data2"]["browserTicket"],return_origin).status == 403
sql(f"UPDATE application SET disable_signin=false WHERE name={quoted(fixture)}")
assert sql("SELECT count(*) FROM record WHERE action='browser-signin' AND object LIKE '%ticket=%'") == "0"
print("PASS embedded: client/origin before OTP, new/old metadata, concurrent single use, browser session, OAuth continuation, disabled app, audit redaction")

# Silent authorization never exposes an interactive hosted page; strict clients cannot redirect elsewhere.
sql(f"UPDATE application SET enable_strict_redirect_uri=true WHERE name={quoted(fixture)}")
silent_params = {"client_id":client_id,"response_type":"code","redirect_uri":callback,"scope":"openid profile","state":"silent & state","prompt":"none"}
anonymous = urllib.request.build_opener(NoRedirect())
def silent(client, params):
    try: return client.open(BASE + "/login/oauth/authorize?" + urllib.parse.urlencode(params))
    except urllib.error.HTTPError as error: return error
r = silent(anonymous, silent_params)
assert r.status == 302
assert urllib.parse.parse_qs(urllib.parse.urlparse(r.headers["Location"]).query) == {"error":["login_required"],"state":["silent & state"]}
for bad in ["http://127.0.0.1:29999/callback", "https://evil.test/?next=" + callback]:
    assert silent(anonymous, {**silent_params,"redirect_uri":bad}).status == 400
r = silent(consumer, {**silent_params,"state":"silent-authenticated"})
assert r.status == 302 and "code=" in r.headers["Location"]
print("PASS silent OIDC: anonymous login_required, encoded state, strict callback rejection, authenticated code")
