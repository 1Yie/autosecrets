import { expect, test } from "@playwright/test";

test("TOTP enrollment dialog does not scroll horizontally", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  const username = "a".repeat(64);

  await page.route("**/api/v1/me", (route) =>
    route.fulfill({
      json: {
        bootstrap_required: false,
        organization: { display_name: "AutoSecrets" },
        member: { id: "member-1", username, role: "administrator" },
        csrf_token: "csrf-token",
      },
    }),
  );
  await page.route("**/api/v1/auth/security", (route) =>
    route.fulfill({
      json: {
        totp_login_required: false,
        oidc: { available: false, bound: false },
      },
    }),
  );
  await page.route("**/api/v1/auth/totp/enrollment", (route) =>
    route.fulfill({
      status: 201,
      json: {
        username,
        enrollment_token: "enrollment-token",
        totp_uri: `otpauth://totp/AutoSecrets%3A${username}?algorithm=SHA1&digits=6&issuer=AutoSecrets&period=30&secret=JBSWY3DPEHPK3PXP`,
      },
    }),
  );

  await page.goto("/dashboard/settings");
  await page.getByRole("tab", { name: "TOTP" }).click();
  await page.getByTestId("local-password").fill("correct-horse-42");
  await page.getByRole("button", { name: "启用本地 TOTP" }).click();

  const dialog = page.getByRole("dialog");
  await expect(
    dialog.getByRole("heading", { name: "验证动态验证码" }),
  ).toBeVisible();
  await expect(
    dialog.getByText(/在身份验证器中添加以下 TOTP 条目/),
  ).toHaveCount(0);
  await expect(dialog.getByLabel("动态验证码")).toBeVisible();
  await dialog.getByText("无法扫码？手动输入").click();

  const viewport = dialog.locator('[data-slot="scroll-area-viewport"]');
  const manualURI = dialog.locator("details code");
  await expect(manualURI).toBeVisible();
  await dialog.getByLabel("动态验证码").click();
  await expect
    .poll(() =>
      viewport.evaluate((element) => element.scrollWidth - element.clientWidth),
    )
    .toBe(0);
  await expect
    .poll(() =>
      manualURI.evaluate(
        (element) => element.scrollWidth - element.clientWidth,
      ),
    )
    .toBeLessThanOrEqual(0);
  await expect(
    dialog.locator(
      '[data-slot="scroll-area-scrollbar"][data-orientation="horizontal"]',
    ),
  ).toHaveCount(0);
});
