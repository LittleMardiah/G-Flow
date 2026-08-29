# PRD (Product Requirement Document) - TEMPLATE
**For Solo Developer - Production Grade**

---

## 📌 HEADER & METADATA

| Field | Value |
|-------|-------|
| **Project Name** | [Your Project Name] |
| **Document Version** | [v1.0] |
| **Created Date** | [YYYY-MM-DD] |
| **Last Updated** | [YYYY-MM-DD] |
| **Author** | [Your Name] |
| **Status** | [Draft / In Review / Approved] |

---

## 📋 EXECUTIVE SUMMARY

**Problem Statement**
[1-2 sentences] What problem does this project solve? Why does it matter?

**Solution Overview**
[1-2 sentences] What are you building? Key differentiators?

**Value Proposition**
[1-2 sentences] Why is this better than alternatives? Who benefits?

**Target Users**
[Brief] Who will use this product? (e.g., "Clinic receptionists", "Dental clinic owners")

---

## 🎯 OBJECTIVES & SUCCESS CRITERIA

### Primary Objectives
- [Objective 1]
- [Objective 2]
- [Objective 3]

### Success Metrics (Must be measurable)
| Metric | Target | Acceptance |
|--------|--------|-----------|
| [Metric name] | [Target value] | [How to measure] |
| [Example: User signup time] | [Example: <2 minutes] | [Example: User analytics] |

---

## 📦 SCOPE DEFINITION

### In Scope (V1.0 MVP - MUST have)
| Feature ID | Feature Name | Priority | Dependency |
|------------|-------------|----------|------------|
| F001 | [Feature 1] | P0 | None |
| F002 | [Feature 2] | P0 | F001 |
| F003 | [Feature 3] | P0 | None |

*P0 = Critical for MVP, must launch*

### Nice-to-Have (V1.1+)
| Feature ID | Feature Name | Estimated Version |
|------------|-------------|-------------------|
| F004 | [Feature 4] | v1.1 |
| F005 | [Feature 5] | v1.2 |

### Out of Scope (Explicitly excluded)
- [Feature/Requirement that will NOT be built in V1]
- [Another excluded feature]

**Rationale:** [Why these are excluded - timeline, complexity, priority]

---

## 👥 USER STORIES & ACCEPTANCE CRITERIA

### User Story 1
```
AS A [user role]
I WANT [action/feature]
SO THAT [business value]

Acceptance Criteria:
- [ ] [Testable criterion 1]
- [ ] [Testable criterion 2]
- [ ] [Testable criterion 3]

Estimated Effort: [T-shirt size: XS/S/M/L/XL]
Priority: P0
```

### User Story 2
```
AS A [user role]
I WANT [action/feature]
SO THAT [business value]

Acceptance Criteria:
- [ ] [Testable criterion 1]
- [ ] [Testable criterion 2]

Estimated Effort: [T-shirt size]
Priority: P[0/1/2]
```

**Note:** Repeat for each major feature. Keep it lean.

---

## 📐 FUNCTIONAL REQUIREMENTS (Detailed)

### Requirement F001: [Feature Name]
**Description:** [What does this feature do?]

**Inputs:**
- [Input 1: type, constraints]
- [Input 2: type, constraints]

**Process:**
1. [Step 1]
2. [Step 2]
3. [Step 3]

**Outputs:**
- [Output 1: format, validation]
- [Output 2: format, validation]

**Edge Cases:**
- [Edge case 1 → Expected behavior]
- [Edge case 2 → Expected behavior]

---

### Requirement F002: [Feature Name]
[Repeat format above]

---

## ⚙️ NON-FUNCTIONAL REQUIREMENTS (NFR)

### Performance
- **Response Time:** [Target] (e.g., "API responses <300ms p95")
- **Page Load:** [Target] (e.g., "Frontend load <2s")
- **Throughput:** [Target] (e.g., "Handle 100 concurrent users")

### Security
- **Authentication:** [Method] (e.g., "JWT tokens")
- **Authorization:** [Model] (e.g., "Role-based access control")
- **Data Protection:** [Standards] (e.g., "Password: bcrypt, data: HTTPS only")
- **Input Validation:** [Approach] (e.g., "Whitelist validation, sanitize all inputs")

### Reliability & Availability
- **Target Uptime:** [Percentage] (e.g., "99% uptime")
- **Data Backup:** [Strategy] (e.g., "Daily automated backups")
- **Disaster Recovery:** [Plan] (e.g., "RTO 1 hour, RPO 15 min")

### Scalability
- **Horizontal Scaling:** [Approach if needed]
- **Database Scaling:** [Strategy]
- **Caching:** [Where applicable]

### Usability
- **Responsive Design:** [Browser/device targets] (e.g., "Chrome, Safari, mobile 375px+")
- **Accessibility:** [Standards] (e.g., "WCAG 2.1 Level AA")
- **User Testing:** [Plan] (e.g., "5 user testers, completion rate >80%")

---

## 🔗 INTEGRATION & DEPENDENCIES

### External Services
| Service | Purpose | Provider | Criticality |
|---------|---------|----------|------------|
| [Service name] | [Purpose] | [Provider] | [Critical/Important/Optional] |

### Internal Dependencies
- [Dependency 1] - required before this project starts
- [Dependency 2] - blocks feature F001

### Technology Constraints
- [Language/Framework constraint]
- [Database constraint]
- [Infrastructure constraint]

---

## ⚠️ ASSUMPTIONS & CONSTRAINTS

### Assumptions
- [Assumption 1] (e.g., "Users have stable internet connection")
- [Assumption 2] (e.g., "Clinic staff trained on system")

### Constraints
- **Timeline:** [Deadline/sprint] (e.g., "Must launch by Q2 2025")
- **Budget:** [Budget info if applicable]
- **Technical:** [Tech constraints] (e.g., "No real-time WebSocket in V1")
- **Resource:** [Team size/availability]

---

## 🚨 RISKS & MITIGATION

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|-----------|
| [Risk description] | High/Med/Low | High/Med/Low | [How to reduce] |
| [Example: Data loss] | Low | Critical | Daily backups, replication |
| [Example: Slow performance] | Medium | High | Caching layer, DB optimization |

---

## 📚 DOCUMENT REFERENCES

Link to related documents in your pipeline:

- **Logic Flow:** [Link to LOGIC_FLOW.md]
- **Page Design (HALAMAN):** [Link to HALAMAN.txt]
- **Design System:** [Link to DESIGN.md]
- **Wireframes/Prototypes:** [Link to Figma/Design files]
- **Technical Design (TDD):** [Link to TDD.md] *(written after this PRD)*
- **API Contract:** [Link to API_CONTRACT.md] *(written after TDD)*

---

## 📊 ACCEPTANCE & SIGN-OFF

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Product Manager | [You] | [YYYY-MM-DD] | ✅ |
| Lead Engineer | [You/Stakeholder] | [YYYY-MM-DD] | ⏳ |

**Notes from Review:**
- [Note 1]
- [Note 2]

---

## 📝 APPENDIX: GLOSSARY (Optional)

| Term | Definition |
|------|-----------|
| [Term 1] | [Definition] |
| [Term 2] | [Definition] |

---

## 📌 VERSION HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | [YYYY-MM-DD] | [Name] | Initial draft |
| 1.1 | [YYYY-MM-DD] | [Name] | Added F005, adjusted scope |

---

## 🎓 USAGE NOTES FOR SOLO DEVELOPER

1. **Keep it concise:** 2-3 pages MAX. Remove any section that doesn't drive decisions.
2. **Before you code:** Approve this PRD with yourself. Lock scope once approved.
3. **After approval:** Move to Logic Flow → HALAMAN → TDD pipeline.
4. **Review periodically:** Only update if scope changes significantly (major feature add/removal).
5. **Share in portfolio:** Include PRD in your GitHub `/docs/PRD.md` as part of delivery.

**Golden Rule:** If you can't explain your feature in 2-3 sentences, you're not ready to code it.

---

*Template Version: 1.0 | Created for DentFlow & Portfolio Projects*
