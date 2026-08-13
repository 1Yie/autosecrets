import {
  createHash,
  createSign,
  generateKeyPairSync,
  randomBytes,
} from "node:crypto";
import { createServer } from "node:http";

const host = "127.0.0.1";
const port = Number(process.env.E2E_OIDC_PORT ?? 19090);
const issuer = `http://${host}:${port}`;
const clientID = "autosecrets-e2e";
const keyID = "e2e-signing-key";
const { privateKey, publicKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
const publicJWK = publicKey.export({ format: "jwk" });
const codes = new Map();
const metrics = { authorization_count: 0, token_count: 0, logout_count: 0 };

const base64url = (value) => Buffer.from(value).toString("base64url");
const json = (response, status, body) => {
  response.writeHead(status, { "Content-Type": "application/json", "Cache-Control": "no-store" });
  response.end(JSON.stringify(body));
};

function idToken(nonce) {
  const now = Math.floor(Date.now() / 1000);
  const header = base64url(JSON.stringify({ alg: "RS256", kid: keyID, typ: "JWT" }));
  const claims = base64url(JSON.stringify({
    iss: issuer,
    sub: "provider-administrator-1",
    aud: clientID,
    exp: now + 300,
    iat: now,
    nonce,
    name: "E2E Administrator",
  }));
  const input = `${header}.${claims}`;
  const signer = createSign("RSA-SHA256");
  signer.update(input);
  signer.end();
  return `${input}.${signer.sign(privateKey).toString("base64url")}`;
}

async function formBody(request) {
  const chunks = [];
  for await (const chunk of request) chunks.push(chunk);
  return new URLSearchParams(Buffer.concat(chunks).toString("utf8"));
}

const server = createServer(async (request, response) => {
  const url = new URL(request.url ?? "/", issuer);
  if (request.method === "GET" && url.pathname === "/.well-known/openid-configuration") {
    json(response, 200, {
      issuer,
      authorization_endpoint: `${issuer}/authorize`,
      token_endpoint: `${issuer}/token`,
      jwks_uri: `${issuer}/jwks`,
      response_types_supported: ["code"],
      id_token_signing_alg_values_supported: ["RS256"],
    });
    return;
  }
  if (request.method === "GET" && url.pathname === "/jwks") {
    json(response, 200, { keys: [{ ...publicJWK, kid: keyID, alg: "RS256", use: "sig" }] });
    return;
  }
  if (request.method === "GET" && url.pathname === "/authorize") {
    const redirectURI = url.searchParams.get("redirect_uri");
    const state = url.searchParams.get("state");
    const nonce = url.searchParams.get("nonce");
    const challenge = url.searchParams.get("code_challenge");
    const valid = url.searchParams.get("client_id") === clientID
      && url.searchParams.get("response_type") === "code"
      && url.searchParams.get("code_challenge_method") === "S256"
      && redirectURI === "http://127.0.0.1:5199/api/v1/auth/oidc/callback"
      && state && nonce && challenge;
    if (!valid) {
      json(response, 400, { error: "invalid_request" });
      return;
    }
    const code = randomBytes(24).toString("base64url");
    codes.set(code, { nonce, challenge, redirectURI });
    metrics.authorization_count++;
    const callback = new URL(redirectURI);
    callback.searchParams.set("code", code);
    callback.searchParams.set("state", state);
    response.writeHead(302, {
      Location: callback.toString(),
      "Set-Cookie": "oidc_test_session=provider-administrator-1; Path=/; HttpOnly; SameSite=Lax",
      "Cache-Control": "no-store",
    });
    response.end();
    return;
  }
  if (request.method === "POST" && url.pathname === "/token") {
    const body = await formBody(request);
    const code = body.get("code");
    const transaction = code ? codes.get(code) : undefined;
    const verifier = body.get("code_verifier") ?? "";
    const challenge = createHash("sha256").update(verifier).digest("base64url");
    const valid = transaction
      && body.get("grant_type") === "authorization_code"
      && body.get("client_id") === clientID
      && body.get("redirect_uri") === transaction.redirectURI
      && challenge === transaction.challenge;
    if (!valid) {
      json(response, 400, { error: "invalid_grant" });
      return;
    }
    codes.delete(code);
    metrics.token_count++;
    json(response, 200, { token_type: "Bearer", id_token: idToken(transaction.nonce) });
    return;
  }
  if (request.method === "GET" && url.pathname === "/metrics") {
    json(response, 200, metrics);
    return;
  }
  if (url.pathname.includes("logout")) metrics.logout_count++;
  json(response, 404, { error: "not_found" });
});

server.listen(port, host, () => process.stdout.write(`OIDC provider listening on ${issuer}\n`));
