# K-0 · Reporting Standards

Findings without reports don't exist.
A well-written report is the deliverable. Everything else is research.

---

## Report Philosophy

**Write for two audiences simultaneously:**
1. A CISO who reads only the executive summary (business risk, headline numbers)
2. A sysadmin who needs to reproduce and fix the finding (exact steps, tool output)

The finding must answer three questions in this order:
1. **What did you find?** (title + description)
2. **What can an attacker do with it?** (impact)
3. **How do you fix it?** (remediation)

If you can't answer all three, it's not ready to report yet.

---

## Finding Structure (Mandatory)

Every finding K-0 generates must include all these fields:

```markdown
### [SEVERITY] Finding Title

**Severity**: CRITICAL / HIGH / MEDIUM / LOW / INFO
**CVSS Score**: [3.1 Base Score if calculable]
**CWE**: CWE-XXX — [CWE Name]
**CVE**: CVE-YYYY-NNNNN (if applicable)
**Target**: [hostname / IP / URL]
**Affected Component**: [service, URL path, parameter, version]

#### Description
Clear technical description of the vulnerability.
What is it? Why does it exist? What does the application do wrong?

#### Evidence
Raw tool output, screenshots, HTTP requests/responses.
This must be reproducible by the client's team.

\`\`\`
[exact tool output / HTTP traffic / proof]
\`\`\`

#### Impact
What can an attacker do with this vulnerability?
Be specific. "Could lead to data breach" is not acceptable.
"Allows unauthenticated retrieval of all user password hashes from /api/users/export" is acceptable.

What's the realistic worst case? What's the most likely exploitation path?

#### Remediation
Specific, actionable fix. Not "improve security practices."

Example:
> Update Apache Struts from 2.3.5 to 2.5.33 or later.
> Apply the vendor patch from: [CVE link]
> If immediate patching is not possible, apply WAF rule to block Content-Type: multipart/form-data
> on endpoints that don't require multipart.

Estimated effort: [Low / Medium / High]

#### References
- CVE link
- Vendor advisory
- OWASP reference
- Proof of concept (if public)
```

---

## Severity Calibration

Use this matrix. Don't guess. Be consistent.

| Severity | CVSS | Definition | Example |
|----------|------|-----------|---------|
| **CRITICAL** | 9.0–10.0 | Immediate exploitation, no auth, full impact | Unauthenticated RCE on internet-facing host |
| **HIGH** | 7.0–8.9 | Significant impact, low complexity | Auth SQLi leading to DB dump, SSRF to cloud credentials |
| **MEDIUM** | 4.0–6.9 | Meaningful impact but limited scope/requires chaining | Stored XSS (no account takeover), IDOR for non-sensitive data |
| **LOW** | 0.1–3.9 | Minimal practical impact, defence-in-depth | Missing security headers, verbose error messages |
| **INFO** | N/A | No direct exploitability, configuration observation | Software version disclosure with no known CVE |

### Severity Adjustment Rules:
- **Increase severity** if: exploitation is proven (PoC works), data is sensitive (PII/credentials/financial), impact is on production
- **Decrease severity** if: requires complex preconditions, internal-only, mitigating controls detected
- **When in doubt, go lower** — overstating severity destroys credibility

---

## Report Structure

### Provisional Report (K-0 generates in-session)

```markdown
# Provisional Engagement Report
**Engagement ID**: [UUID]
**Target**: [targets in scope]
**Generated**: [timestamp]
**Status**: PROVISIONAL — requires analyst review before delivery

---

## Executive Summary
[2-3 paragraphs: what was tested, headline findings, overall risk posture]

## Risk Summary
| Severity | Count |
|----------|-------|
| Critical | X |
| High | X |
| Medium | X |
| Low | X |
| Info | X |

## Findings
[One section per finding, using Finding Structure above]

## Appendix A — Tools Used
[List of tools, commands, timestamps]

## Appendix B — Scope
[What was in scope, what was excluded, dates]
```

---

## Writing Style Rules

**Be specific:**
- ❌ "The application is vulnerable to SQL injection."
- ✅ "The `id` parameter in `/api/products?id=` is vulnerable to time-based blind SQL injection. Evidence: a `SLEEP(5)` payload caused a 5-second response delay."

**Evidence first:**
- Include the exact HTTP request/response, tool output, or command that proves the finding
- Don't describe what you saw — show it

**Quantify impact:**
- ❌ "Could expose sensitive user data"
- ✅ "The `/api/users/export` endpoint returned 14,000 user records including emails, hashed passwords (MD5 unsalted), and home addresses"

**Remediation must be actionable:**
- ❌ "Implement proper input validation"
- ✅ "Use parameterised queries (prepared statements) instead of string concatenation. In Python: `cursor.execute('SELECT * FROM users WHERE id = %s', (user_id,))`"

---

## Common Mistakes to Avoid

### In findings:
- Reporting without evidence (no screenshot, no tool output)
- Mixing multiple vulnerabilities into one finding
- Saying "critical" on a theoretical issue with no proven exploitation path
- Forgetting to note if findings are confirmed or suspected
- Missing remediation specifics

### In the report:
- Writing for only one audience (too technical OR too vague)
- No executive summary
- CVSS scores without justification
- Inconsistent severity across similar findings
- No scope section (what was NOT tested?)
- Report delivered without a re-test recommendation timeline

---

## CVSS 3.1 Quick Calculator

**Base Score components:**

| Metric | Options |
|--------|--------|
| Attack Vector | Network(N) / Adjacent(A) / Local(L) / Physical(P) |
| Attack Complexity | Low(L) / High(H) |
| Privileges Required | None(N) / Low(L) / High(H) |
| User Interaction | None(N) / Required(R) |
| Scope | Unchanged(U) / Changed(C) |
| Confidentiality | None(N) / Low(L) / High(H) |
| Integrity | None(N) / Low(L) / High(H) |
| Availability | None(N) / Low(L) / High(H) |

**Quick scores for common findings:**
- Unauth RCE over network: AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H → **10.0 CRITICAL**
- Auth SQLi data dump: AV:N/AC:L/PR:L/UI:N/S:U/C:H/I:H/A:N → **8.1 HIGH**
- Reflected XSS needs click: AV:N/AC:L/PR:N/UI:R/S:C/C:L/I:L/A:N → **6.1 MEDIUM**
- Missing security header: → **INFO** (no CVSS)

Use https://www.first.org/cvss/calculator/3.1 for precise scoring.

---

## K-0 Reporting Automation Notes

K-0 auto-generates provisional reports saved to:
`~/.kiai/memory/reports/provisional-YYYY-MM-DD-[goalID].md`

These are **provisional** — human review required before delivery.
K-0 marks unconfirmed findings with `[UNCONFIRMED]`.
K-0 marks auto-extracted findings with `[AUTO-EXTRACTED — verify]`.

After engagement review:
1. Remove `[PROVISIONAL]` header
2. Verify all evidence is attached / screenshots referenced
3. Run CVSS scoring on all HIGH/CRITICAL
4. Add executive summary
5. Review remediation wording
6. Flag items for retest
