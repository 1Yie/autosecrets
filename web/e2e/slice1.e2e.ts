/// <reference types="node" />

import { createHmac } from "node:crypto";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import https from "node:https";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test, type Page } from "@playwright/test";

const PROXY_URL = process.env.E2E_PROXY_URL ?? "https://localhost:18443";
const KEY_DIR = process.env.E2E_KEYS ?? "";
const BOOTSTRAP_CODE = process.env.E2E_BOOTSTRAP_CODE ?? "";
const PASSWORD = "correct-horse-42";

test("single-Administrator authentication and Secret delivery journey", async ({
	context,
	page,
	request,
}) => {
	test.skip(!KEY_DIR || !BOOTSTRAP_CODE, "E2E stack not configured");

	await page.goto("/");
	await expect(
		page.getByRole("heading", { name: "初始化 AutoSecrets" }),
	).toBeVisible();
	await page.getByTestId("code").fill(BOOTSTRAP_CODE);
	await page.getByTestId("username").fill("admin");
	await page.getByTestId("password").fill(PASSWORD);
	await page.getByRole("button", { name: "创建管理员" }).click();
	await expect(page.getByRole("heading", { name: "概览" })).toBeVisible();

	await page.goto("/dashboard/settings");
	await page.getByRole("tab", { name: "TOTP" }).click();
	await expect(page.getByText("已停用")).toBeVisible();
	await page.getByTestId("local-password").fill(PASSWORD);
	await page.getByRole("button", { name: "启用本地 TOTP" }).click();
	await expect(
		page.getByRole("heading", { name: "验证动态验证码" }),
	).toBeVisible();
	const enrollmentURI = (await page.locator("details code").textContent()) ?? "";
	let secret = "";
	try {
		secret = new URL(enrollmentURI).searchParams.get("secret") ?? "";
	} catch {
		// The URI is emitted by Core; a parse failure already fails the expect below.
	}
	expect(secret).not.toBe("");
	await page.getByTestId("totp-code").fill(totpCode(secret));
	await page.getByRole("button", { name: "验证" }).click();
	await expect(page.getByRole("heading", { name: "保存恢复码" })).toBeVisible();
	const recoveryCodes = await page
		.getByTestId("recovery-codes")
		.locator("li")
		.allTextContents();
	expect(recoveryCodes).toHaveLength(10);
	await page.getByTestId("recovery-ack").check();
	await page.getByRole("button", { name: "完成注册" }).click();
	await expect(page.getByText("已启用")).toBeVisible();

	await logout(page);
	await passwordStep(page);
	const loginCounter = totpCounter();
	await page.getByTestId("totp-code").fill(totpCode(secret));
	await page.getByRole("button", { name: "继续" }).click();
	await expect(page.getByRole("heading", { name: "概览" })).toBeVisible();

	await authenticatedPost(page, "/api/v1/auth/renew", {
		password: PASSWORD,
		recovery_code: recoveryCodes[0],
	});
	await page.reload();
	await expect(page.getByRole("heading", { name: "概览" })).toBeVisible();
	await authenticatedPost(page, "/api/v1/auth/step-up", {
		password: PASSWORD,
		recovery_code: recoveryCodes[1],
	});

	await logout(page);
	await passwordStep(page);
	await page.getByTestId("factor-recovery").click();
	await page.getByTestId("recovery-code").fill(recoveryCodes[2]);
	await page.getByRole("button", { name: "继续" }).click();
	await expect(page.getByRole("heading", { name: "概览" })).toBeVisible();

	let currentCounter = await nextTOTPCounter(loginCounter);
	await page.goto("/dashboard/settings");
	await page.getByRole("tab", { name: "外部登录" }).click();
	await page.getByTestId("oidc-password").fill(PASSWORD);
	await page.getByTestId("oidc-totp").fill(totpCode(secret, currentCounter));
	await page.getByRole("button", { name: "绑定 External Identity" }).click();
	await expect(page).toHaveURL(/\/dashboard\/security$/);
	await expect(page.getByText("E2E Administrator")).toBeVisible();
	await expect(page.getByText("已绑定")).toBeVisible();

	await logout(page);
	await expect(
		page.getByRole("button", { name: "使用 OpenID Connect 登录" }),
	).toBeVisible();
	await page.getByRole("button", { name: "使用 OpenID Connect 登录" }).click();
	await expect(page.getByRole("heading", { name: "概览" })).toBeVisible();
	const oidcMe = await page.evaluate(async () =>
		(await fetch("/api/v1/me")).json(),
	);
	expect(oidcMe.auth_method).toBe("oidc");
	await authenticatedPost(page, "/api/v1/auth/renew", {
		password: PASSWORD,
		recovery_code: recoveryCodes[3],
	});
	await page.reload();

	const metricsBeforeLogout = await (
		await request.get("http://127.0.0.1:19090/metrics")
	).json();
	await logout(page);
	const providerCookies = await context.cookies("http://127.0.0.1:19090");
	expect(
		providerCookies.some((cookie) => cookie.name === "oidc_test_session"),
	).toBe(true);
	const metricsAfterLogout = await (
		await request.get("http://127.0.0.1:19090/metrics")
	).json();
	expect(metricsAfterLogout.logout_count).toBe(metricsBeforeLogout.logout_count);

	await passwordStep(page);
	await page.getByTestId("factor-recovery").click();
	await page.getByTestId("recovery-code").fill(recoveryCodes[4]);
	await page.getByRole("button", { name: "继续" }).click();
	currentCounter = await nextTOTPCounter(currentCounter);
	await page.goto("/dashboard/settings");
	await page.getByRole("tab", { name: "外部登录" }).click();
	await page.getByTestId("oidc-password").fill(PASSWORD);
	await page.getByTestId("oidc-totp").fill(totpCode(secret, currentCounter));
	await page.getByRole("button", { name: "解除绑定" }).click();
	await expect(page.getByText("未绑定")).toBeVisible();

	currentCounter = await nextTOTPCounter(currentCounter);
	await page.getByRole("tab", { name: "TOTP" }).click();
	await page.getByTestId("local-password").fill(PASSWORD);
	await page.getByTestId("local-totp").fill(totpCode(secret, currentCounter));
	await page.getByRole("button", { name: "停用本地 TOTP" }).click();
	await expect(page.getByText("已停用")).toBeVisible();

	await logout(page);
	await page.getByTestId("username").fill("admin");
	await page.getByTestId("password").fill(PASSWORD);
	await page.getByRole("button", { name: "登录", exact: true }).click();
	await expect(page.getByRole("heading", { name: "概览" })).toBeVisible();
	await expect(
		page.getByRole("button", { name: "使用 OpenID Connect 登录" }),
	).toHaveCount(0);

	await secretDeliveryJourney(page);
});

async function passwordStep(page: Page) {
	await page.getByTestId("username").fill("admin");
	await page.getByTestId("password").fill(PASSWORD);
	await page.getByRole("button", { name: "登录", exact: true }).click();
	await expect(
		page.getByRole("heading", { name: "验证第二因子" }),
	).toBeVisible();
}

async function logout(page: Page) {
	await page.getByTestId("current-user").click();
	await page.getByTestId("logout").click();
	await expect(
		page.getByRole("heading", { name: "登录", exact: true }),
	).toBeVisible();
}

async function authenticatedPost(
	page: Page,
	path: string,
	body: Record<string, string>,
) {
	const result = await page.evaluate(
		async ({ path, body }) => {
			const me = await (await fetch("/api/v1/me")).json();
			const response = await fetch(path, {
				method: "POST",
				credentials: "same-origin",
				headers: {
					"Content-Type": "application/json",
					"X-CSRF-Token": me.csrf_token,
				},
				body: JSON.stringify(body),
			});
			return { status: response.status, body: await response.json() };
		},
		{ path, body },
	);
	expect(result.status, JSON.stringify(result.body)).toBe(200);
}

function totpCounter(now = Date.now()) {
	return Math.floor(now / 30_000);
}

async function nextTOTPCounter(previous: number) {
	while (totpCounter() <= previous)
		await new Promise((resolve) => setTimeout(resolve, 250));
	return totpCounter();
}

function totpCode(secret: string, counter = totpCounter()) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
	let bits = "";
	for (const character of secret.toUpperCase().replaceAll("=", "")) {
		bits += alphabet.indexOf(character).toString(2).padStart(5, "0");
	}
	const key = Buffer.alloc(Math.floor(bits.length / 8));
	for (let index = 0; index < key.length; index++)
		key[index] = Number.parseInt(bits.slice(index * 8, index * 8 + 8), 2);
	const message = Buffer.alloc(8);
	message.writeBigUInt64BE(BigInt(counter));
	const digest = createHmac("sha1", key).update(message).digest();
	const offset = digest[digest.length - 1] & 0x0f;
	const value = (digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000;
	return value.toString().padStart(6, "0");
}

async function secretDeliveryJourney(page: Page) {
	const nodeName = "e2e-node";
	const prefix = mkdtempSync(join(tmpdir(), "as-node-"));
	const bundleDir = join(prefix, "bundles");
	await page.goto("/dashboard/apps");
	await page.getByRole("button", { name: "新建应用" }).click();
	await page.getByTestId("app-name").fill("payments");
	await page.getByRole("button", { name: "创建", exact: true }).click();
	await page.getByRole("link", { name: /payments/ }).click();
	await page.getByRole("button", { name: "新建环境" }).click();
	await page.getByTestId("env-name").fill("production");
	await page.getByRole("button", { name: "创建", exact: true }).click();
	await page.getByTestId("env-production").click();
	await page.getByRole("button", { name: "添加密钥" }).click();
	await page.getByTestId("secret-name").fill("db_pass");
	await page.getByTestId("secret-value").fill("e2e-secret-value-1");
	await page.getByRole("button", { name: "添加", exact: true }).click();
	await page.getByRole("button", { name: "发布" }).click();
	await expect(page.getByText("已发布，节点将自动同步")).toBeVisible();

	await page.goto("/dashboard/nodes");
	await page.getByRole("tab", { name: "节点组" }).click();
	await page.getByRole("button", { name: "新建节点组" }).click();
	await page.getByTestId("node-group-name").fill("g1");
	await page.getByRole("button", { name: "创建", exact: true }).click();
	await page.getByRole("tab", { name: "托管节点" }).click();
	await page.getByRole("button", { name: "添加服务器" }).click();
	await page.getByTestId("node-name").fill(nodeName);
	await page.getByTestId("node-bundle-dir").fill(bundleDir);
	await page.getByRole("button", { name: "添加", exact: true }).click();
	await expect(page.getByText("服务器已添加")).toBeVisible();
	await page.getByRole("button", { name: "生成连接" }).click();
	const command =
		(await page.getByTestId("install-command").textContent()) ?? "";
	expect(command).toContain(`--bundle-dir "${bundleDir}"`);
	const token = command.split("--token ")[1].split(/\s+/)[0];

	const script = await new Promise<string>((resolve, reject) => {
		https
			.get(
				`${PROXY_URL}/agent/v1/install.sh`,
				{ ca: readFileSync(join(KEY_DIR, "agent-ca.crt")) },
				(res) => {
					let data = "";
					res.on("data", (chunk) => (data += chunk));
					res.on("end", () => resolve(data));
				},
			)
			.on("error", reject);
	});
	writeFileSync(join(prefix, "install.sh"), script);
	execFileSync(
		"bash",
		[
			join(prefix, "install.sh"),
			"--server",
			PROXY_URL,
			"--token",
			token,
			"--name",
			nodeName,
			"--bundle-dir",
			bundleDir,
		],
		{
			env: {
				...process.env,
				AUTOSECRETS_PREFIX: join(prefix, "opt"),
				AUTOSECRETS_CONFIG_DIR: join(prefix, "etc"),
				AUTOSECRETS_STATE_DIR: join(prefix, "var"),
				AUTOSECRETS_CURL_OPTS: "-k",
				AUTOSECRETS_NO_SYSTEMD: "1",
			},
			timeout: 120_000,
			stdio: "pipe",
		},
	);

	await page.reload();
	await page.getByRole("tab", { name: "节点组" }).click();
	await page.getByRole("button", { name: "管理 g1" }).click();
	await page.getByRole("combobox", { name: "添加节点" }).click();
	const memberAdded = page.waitForResponse(
		(response) =>
			response.request().method() === "POST" &&
			/\/api\/v1\/node-groups\/[^/]+\/nodes$/.test(response.url()),
	);
	await page.getByRole("option", { name: nodeName }).click();
	expect((await memberAdded).status()).toBe(200);
	await page.keyboard.press("Escape");
	await page.getByRole("tab", { name: "分配" }).click();
	await page.getByTestId("assignment-application").click();
	await page.getByRole("option", { name: "payments" }).click();
	await page.getByTestId("assignment-environment").click();
	await page.getByRole("option", { name: /production/ }).click();
	const assignmentCreated = page.waitForResponse(
		(response) =>
			response.request().method() === "POST" &&
			response.url().endsWith("/api/v1/assignments"),
	);
	await page.getByRole("button", { name: "绑定应用" }).click();
	expect((await assignmentCreated).status()).toBe(201);
	await expect(page.getByText("该组还没有绑定任何应用。")).toHaveCount(0);
	await page.getByRole("button", { name: "完成" }).click();

	const agent = join(prefix, "opt", "autosecrets-agent");
	const syncOutput = execFileSync(
		agent,
		["sync", "--config", join(prefix, "etc", "config.toml")],
		{
			timeout: 60_000,
			stdio: "pipe",
		},
	).toString();
	const files: string[] = [];
	const walk = (directory: string) => {
		for (const entry of readdirSync(directory, { withFileTypes: true })) {
			const full = join(directory, entry.name);
			if (entry.isDirectory()) walk(full);
			else files.push(full);
		}
	};
	walk(bundleDir);
	const secretFile = files.find((file) => file.endsWith("/db_pass"));
	expect(
		secretFile,
		`sync output: ${syncOutput}; bundle files: ${files.join(", ")}`,
	).toBeTruthy();
	expect(readFileSync(secretFile!, "utf8")).toBe("e2e-secret-value-1");
}
