# Disaster Recovery Plan (DRP) Template

> **Purpose**: Dokumen ini adalah template untuk membuat Disaster Recovery Plan yang sistematis. DRP adalah prosedur dan dokumentasi tentang cara memulihkan seluruh sistem setelah bencana, mencakup worst-case scenarios dan solusi preventif.

---

## 1. EXECUTIVE SUMMARY

### 1.1 Project Information
- **Project Name**: [Nama Aplikasi/Sistem]
- **Owner/Team**: [Nama Tim / Person Responsible]
- **Last Updated**: [DD/MM/YYYY]
- **Next Review Date**: [DD/MM/YYYY]
- **Version**: [1.0]

### 1.2 DRP Objectives
- Meminimalkan downtime sistem saat terjadi disaster
- Melindungi integritas dan keamanan data
- Memastikan business continuity
- Menetapkan clear roles dan responsibilities

### 1.3 Key Metrics
| Metric | Value | Justification |
|--------|-------|---------------|
| RTO (Recovery Time Objective) | [X jam/menit] | Waktu maksimal sistem boleh down |
| RPO (Recovery Point Objective) | [X menit/jam] | Data maksimal yang boleh hilang |
| Backup Frequency | [X kali per hari/minggu] | Sesuai dengan volume transaksi |

---

## 2. SCOPE & SYSTEMS INVENTORY

### 2.1 In Scope
Dokumentasikan sistem/komponen yang covered oleh DRP ini:
- [ ] Database (Primary & Replica)
- [ ] Application Servers
- [ ] API Services
- [ ] Message Queues / Event Streams
- [ ] External Integrations (Payment Gateway, SMS Service, etc.)
- [ ] File Storage / Cloud Storage
- [ ] Authentication & Authorization System
- [ ] Caching Layer (Redis, Memcached, etc.)
- [ ] Load Balancers & Network Infrastructure
- [ ] Configuration & Secrets Management

### 2.2 Out of Scope
Sistem/komponen yang **tidak** tercover DRP ini (beserta alasan):
- [Sistem/Komponen]: [Alasan]
- [Sistem/Komponen]: [Alasan]

### 2.3 Infrastructure Overview
Berikan deskripsi singkat arsitektur sistem:
```
[Diagrams atau text description dari topology]

Contoh format text:
- Web Layer: [Load Balancer] → [N Application Servers]
- Database Layer: [Primary DB] ↔ [Standby DB] + [Read Replicas]
- Cache Layer: [Redis Cluster]
- External: [API Gateway] → [Third-party Services]
```

---

## 3. DISASTER SCENARIOS & RISK ASSESSMENT

### 3.1 Disaster Scenario Matrix

Definisikan skenario worst-case yang mungkin terjadi, dengan severity level:

| # | Scenario | Severity | RTO | RPO | Likelihood | Impact |
|---|----------|----------|-----|-----|------------|--------|
| 1 | [Scenario A] | Critical | [X] | [Y] | [High/Med/Low] | [Business Impact] |
| 2 | [Scenario B] | High | [X] | [Y] | [High/Med/Low] | [Business Impact] |
| 3 | [Scenario C] | Medium | [X] | [Y] | [High/Med/Low] | [Business Impact] |

### 3.2 Severity Levels Definition

**CRITICAL** (RTO: < 1 jam, RPO: 0 data loss)
- Skenario yang langsung mengancam core business operations
- Data loss tidak dapat diterima
- Contoh worst cases:
  - Primary database complete failure / data corruption
  - Authentication system unavailable
  - Payment processing system down (untuk e-commerce)
  - Patient/Customer data exposure (compliance breach)

**HIGH** (RTO: 2-4 jam, RPO: ≤ 1 jam)
- Skenario yang significant impact tapi tidak immediate threat ke core business
- Beberapa data loss dapat direcovery melalui alternative means
- Contoh worst cases:
  - Read replicas down
  - API service unavailable
  - Scheduled service failures
  - Performance degradation

**MEDIUM** (RTO: 4-24 jam, RPO: ≤ 4 jam)
- Skenario yang impact terbatas atau partial functionality loss
- Data loss minimal atau tidak critical
- Contoh worst cases:
  - Admin portal down
  - Analytics/reporting service unavailable
  - Non-critical microservice failure
  - Cache layer failure (dengan fallback)

**LOW** (RTO: > 24 jam, RPO: ≤ 1 hari)
- Skenario dengan minimal business impact
- Dapat di-defer tanpa significant loss
- Contoh worst cases:
  - Development environment down
  - Non-customer-facing service
  - Cosmetic UI issues

---

## 4. BACKUP & REDUNDANCY STRATEGY

### 4.1 Backup Architecture

#### 4.1.1 Database Backups
```
Strategy: [Full/Incremental/Differential]
- Full Backup Frequency: [X times per day/week]
- Incremental/Differential Frequency: [X times per day]
- Retention Period: [X days/weeks/months]
- Storage Location: [Primary Location], [Secondary Location (different region)]
- Verification: [Automated restore test frequency]
```

**Implementation Details**:
- Backup Tool/Method: [e.g., mysqldump, pg_dump, AWS RDS snapshots, etc.]
- Backup Window: [Time range ketika backup dijalankan]
- Backup Size Estimation: [Approx. XX GB per day]
- Storage Cost: [Estimate biaya penyimpanan backup]

#### 4.1.2 Application & Code Backups
```
- Version Control: [GitHub, GitLab, Bitbucket, etc.]
- Backup Frequency: [Real-time (via VCS)]
- Repository Mirroring: [Primary], [Mirror Location]
- Configuration Files: [Backup location & frequency]
- Secrets Management: [Encrypted vault, location, backup strategy]
```

#### 4.1.3 File Storage / Object Storage Backups
```
- File Type: [Documents, Images, Uploads, etc.]
- Backup Strategy: [Cross-region replication, versioning, etc.]
- Frequency: [Real-time / Hourly / Daily]
- Retention: [X versions / X days]
```

### 4.2 Redundancy & High Availability

| Component | Redundancy Type | Implementation | Failover Time |
|-----------|-----------------|-----------------|---------------|
| [Component A] | [Active-Active / Active-Passive] | [Details] | [X seconds] |
| [Component B] | [Active-Active / Active-Passive] | [Details] | [X seconds] |

**Failover Mechanisms**:
- Load Balancer: [Type & configuration]
- Database Replication: [Streaming / Log-based / Snapshot-based]
- DNS Failover: [Automatic / Manual]
- Health Checks: [Frequency & criteria]

### 4.3 Geographic Distribution
```
Primary Region: [Location]
- Data Center / Cloud Region: [Details]
- Backup Database: [Synchronization method & lag]

Secondary Region: [Location]
- Data Center / Cloud Region: [Details]
- Purpose: [Warm standby / Cold standby / Active-active]
- RTO if Primary Fails: [X minutes]
```

---

## 5. DISASTER RESPONSE PLAYBOOKS

### 5.1 Generic Playbook Structure

Setiap disaster scenario harus memiliki playbook dengan struktur berikut:

#### **[SCENARIO NAME]**

**Severity**: [Critical / High / Medium / Low]

**Symptoms / Detection**:
- [Alert/Signal #1]: [How detected, Threshold]
- [Alert/Signal #2]: [How detected, Threshold]

**Impact Assessment**:
- Affected Systems: [List]
- User Impact: [Description]
- Business Impact: [Financial/Operational impact]

**Immediate Actions (First 15 minutes)**:
1. [Action 1]: [Who performs, Tool used, Expected outcome]
2. [Action 2]: [Who performs, Tool used, Expected outcome]
3. [Action 3]: [Who performs, Tool used, Expected outcome]

**Recovery Steps (Detailed)**:
1. **Preparation Phase**:
   - [ ] Notify [Role]
   - [ ] Activate [System/Tool]
   - [ ] Verify [Condition]

2. **Execution Phase**:
   ```bash
   # Command/Script 1
   [step-by-step instructions]
   
   # Command/Script 2
   [step-by-step instructions]
   ```

3. **Validation Phase**:
   - [ ] Check [Condition 1]: [Expected result]
   - [ ] Check [Condition 2]: [Expected result]
   - [ ] Run [Test/Query]: [Expected result]

4. **Post-Recovery**:
   - [ ] Verify data integrity: [How to verify]
   - [ ] Monitor [Metrics] for X minutes
   - [ ] Document incident in [Location]
   - [ ] Notify stakeholders of recovery

**Rollback Plan** (if recovery fails):
- Condition to trigger rollback: [Criteria]
- Rollback steps: [Step-by-step]

**Expected RTO**: [X hours/minutes]

**Expected RPO**: [X hours/minutes]

---

### 5.2 Playbook Template Examples

**Example 1: Database Corruption**

**Severity**: Critical

**Symptoms / Detection**:
- Alert: Database consistency check fails
- Error logs: [Specific error pattern]

**Impact Assessment**:
- Affected Systems: Application Layer, API Layer
- User Impact: Complete service unavailability
- Business Impact: Revenue loss, SLA breach

**Immediate Actions (First 15 minutes)**:
1. Confirm the issue via monitoring dashboard
2. Notify incident commander & database team
3. Stop application servers to prevent further corruption
4. Prepare for recovery from backup

**Recovery Steps**:
1. **Preparation**:
   - [ ] Identify which backup to restore from (based on RPO)
   - [ ] Verify backup integrity
   - [ ] Allocate resources (servers, storage)

2. **Execution**:
   ```
   Step 1: Stop all application services
   Step 2: Shutdown database instance
   Step 3: Restore from backup [Backup ID]
   Step 4: Verify data consistency post-restore
   Step 5: Start database instance
   Step 6: Run smoke tests
   Step 7: Restart application servers
   ```

3. **Validation**:
   - [ ] Database health check passes
   - [ ] Core queries return expected results
   - [ ] Application services become healthy

4. **Post-Recovery**:
   - [ ] Monitor for 1 hour with extra attention
   - [ ] Identify root cause
   - [ ] Create incident report

**Rollback Plan**:
- If recovery fails after 30 minutes: Switch to secondary region backup

**Expected RTO**: 1-2 hours | **Expected RPO**: 0-15 minutes

---

**Example 2: API Service Unavailable**

**Severity**: High

**Symptoms / Detection**:
- Alert: API response time > 30s
- Error rate: > 10% of requests
- Health check: Service not responding

**Impact Assessment**:
- Affected Systems: API Gateway, dependent microservices
- User Impact: Slow service / errors for users
- Business Impact: Reduced conversion, user frustration

**Immediate Actions**:
1. Check API service status & resource utilization
2. Review recent deployments
3. Initiate automatic failover to secondary instance

**Recovery Steps**:
1. **Diagnosis**:
   ```
   Check: CPU, Memory, Network utilization
   Check: Error logs for patterns
   Check: Recent code changes
   ```

2. **Execution Options**:
   - Option A: Restart service
   - Option B: Rollback recent deployment
   - Option C: Scale up instances (if load-based)
   - Option D: Route traffic to backup API

3. **Validation**:
   - [ ] Response time < 1s
   - [ ] Error rate < 0.1%
   - [ ] All critical endpoints responding

**Expected RTO**: 15-30 minutes | **Expected RPO**: Minimal (stateless service)

---

---

## 6. ROLES, RESPONSIBILITIES & ESCALATION

### 6.1 Incident Response Team

| Role | Responsibilities | Contact | Escalation |
|------|-----------------|---------|-----------|
| **Incident Commander** | Coordinate response, decision-making, communication | [Contact] | [To: CEO/CTO if RTO exceeded] |
| **Database Administrator** | Database recovery, backup management | [Contact] | [To: Incident Commander] |
| **Infrastructure/DevOps** | Server, network, failover operations | [Contact] | [To: Incident Commander] |
| **Application Lead** | Application-specific issues, rollback decisions | [Contact] | [To: Incident Commander] |
| **Communications Lead** | Internal & external communications, status updates | [Contact] | [To: Incident Commander] |

### 6.2 Escalation Path

```
Level 1 (On-Call Engineer) → Level 2 (Tech Lead) → Level 3 (Engineering Manager) → Level 4 (CTO/VP)

Escalation Triggers:
- RTO exceeded by 30 minutes
- Data loss confirmed
- Multiple systems affected
- Potential compliance/legal issue
```

### 6.3 Communication Plan

**Internal Communications**:
- Channel: [Slack channel / War room]
- Update Frequency: Every X minutes (during incident)
- Template: [Status update format]

**External Communications**:
- Customer Notification: [Timing & channel]
- Social Media: [Who manages, messaging guidelines]
- Status Page: [Update frequency]

---

## 7. TESTING, VALIDATION & DRILLS

### 7.1 Testing Strategy

**Backup Restoration Tests**:
- Frequency: [Weekly / Monthly]
- Scope: [Full restore / Partial restore]
- Success Criteria: [Data integrity check, restore time < X hours]
- Owner: [Database team]
- Documentation: [Test results stored in]

**Failover Tests**:
- Frequency: [Quarterly]
- Scope: [Primary → Secondary]
- Success Criteria: [Failover time < RTO, zero data loss]
- Owner: [Infrastructure team]
- Documentation: [Test report template]

**Application Smoke Tests**:
- Frequency: [After every recovery / deployment]
- Scope: [Critical user flows]
- Success Criteria: [All critical endpoints pass]
- Owner: [QA / Application team]

### 7.2 Disaster Recovery Drills

**Monthly Drill** (30 minutes):
- **Scenario**: [Pick one scenario each month]
- **Participants**: [List roles]
- **Goals**: Test playbook, measure response time
- **No-Impact Drill**: Use staging environment or read-only operations

**Quarterly Drill** (2-4 hours):
- **Scenario**: Full system recovery simulation
- **Participants**: Full incident response team
- **Goals**: Test entire DRP, identify gaps
- **Realistic Conditions**: Use production-like data volumes

**Annual Review**:
- Full DRP audit
- Update based on infrastructure changes
- Incorporate lessons learned from incidents

### 7.3 Drill Execution Checklist

```
Pre-Drill:
[ ] Notify all participants
[ ] Prepare staging/safe environment
[ ] Backup current state (if applicable)
[ ] Set drill duration & success criteria

During Drill:
[ ] Start timer
[ ] Execute playbook steps
[ ] Log all actions & timings
[ ] Document issues/blockers

Post-Drill:
[ ] Verify system recovery
[ ] Calculate actual RTO/RPO
[ ] Collect feedback from participants
[ ] Create after-action report
[ ] Update playbook based on findings
```

### 7.4 Metrics & KPIs

Track dari setiap drill/incident:
- **Actual RTO**: [Waktu actual recovery]
- **Actual RPO**: [Data loss actual]
- **Playbook Accuracy**: [% steps yang correct/relevant]
- **Team Response Time**: [Waktu tim respond ke incident]
- **Knowledge Gaps**: [Issues discovered, learning needed]

---

## 8. MAINTENANCE & CONTINUOUS IMPROVEMENT

### 8.1 Regular Maintenance Tasks

| Task | Frequency | Owner | Notes |
|------|-----------|-------|-------|
| Review playbooks | Quarterly | [Role] | Update for new systems/changes |
| Test backup restoration | Weekly/Monthly | [Role] | Verify data integrity |
| Update contact list | Monthly | [Role] | Ensure current phone/email |
| Review RTO/RPO targets | Semi-annually | [Role] | Align with business needs |
| Audit backup storage | Monthly | [Role] | Verify size, retention, security |

### 8.2 Post-Incident Review Process

Setelah terjadi real incident:

```
Within 24 hours:
- Document what happened (timeline, actions taken)
- Identify root cause
- Measure actual RTO/RPO
- Collect team feedback

Within 1 week:
- Complete incident report
- Identify preventive measures
- Update playbooks if needed
- Share lessons learned

Within 30 days:
- Implement preventive measures
- Run drill based on new learnings
- Close incident ticket
```

### 8.3 Version Control & Change Log

| Version | Date | Changes | Author |
|---------|------|---------|--------|
| 1.0 | [DD/MM/YYYY] | Initial DRP creation | [Name] |
| 1.1 | [DD/MM/YYYY] | [Changes] | [Name] |

---

## 9. APPENDIX

### 9.1 Glossary

- **RTO (Recovery Time Objective)**: Waktu maksimal sistem boleh down
- **RPO (Recovery Point Objective)**: Data maksimal yang boleh hilang
- **Backup**: Copy dari data untuk recovery purposes
- **Failover**: Switching to backup systems saat primary fails
- **Disaster**: Event yang cause system/data unavailable (serangan siber, hardware failure, natural disaster, human error, etc.)

### 9.2 Related Documents

- [ ] Infrastructure Documentation: [Link]
- [ ] API Contract / Service Documentation: [Link]
- [ ] Security Policy: [Link]
- [ ] Backup Strategy Document: [Link]
- [ ] Incident Response Policy: [Link]

### 9.3 Tools & Resources

| Tool/Resource | Purpose | Access |
|---------------|---------|--------|
| [Monitoring Tool] | Real-time alerts & observability | [URL/Location] |
| [Backup Tool] | Backup management & restore | [URL/Location] |
| [Incident Tracker] | Document incidents & recovery | [URL/Location] |
| [Communication Tool] | War room & incident coordination | [Slack/Teams/etc] |

### 9.4 Recovery Commands Reference

**Database Recovery** (Template):
```bash
# Stop application services
[Command 1]

# Restore from backup
[Command 2]

# Verify integrity
[Command 3]

# Restart services
[Command 4]
```

**API Failover** (Template):
```bash
# Check health
[Command 1]

# Update DNS / Load Balancer
[Command 2]

# Verify traffic routing
[Command 3]
```

---

## Document Sign-Off

| Role | Name | Date | Signature |
|------|------|------|-----------|
| DRP Owner | [Name] | [Date] | [Sign] |
| Engineering Lead | [Name] | [Date] | [Sign] |
| IT/Infrastructure Lead | [Name] | [Date] | [Sign] |
| Management Approval | [Name] | [Date] | [Sign] |

---

**Last Updated**: [DD/MM/YYYY]  
**Next Review**: [DD/MM/YYYY]  
**Status**: [Draft / Approved / Active]
