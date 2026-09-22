/// Model wallet (PayPulse/F001).
///
/// Disimpan lokal di customer_app (packages/core belum ada di monorepo ini).
/// Field mengikuti response backend wallet. Catatan: GET /wallets/{id}/balance
/// (internal/wallet/handler.go:212) saat ini hanya mengembalikan
/// {wallet_id, balance}; field lain (status/wallet_type/user_id/updated_at)
/// dibuat nullable agar parsing defensif tahan terhadap response yang lebih
/// kaya (API_CONTRACT §6.1) maupun yang minimal.
class Wallet {
  const Wallet({
    required this.walletId,
    this.userId,
    this.balance = 0,
    this.status,
    this.walletType,
    this.updatedAt,
  });

  final String walletId;
  final String? userId;
  final int balance;
  final String? status;
  final String? walletType;
  final DateTime? updatedAt;

  factory Wallet.fromJson(Map<String, dynamic> j) {
    return Wallet(
      walletId: _str(j['wallet_id']) ?? _str(j['id']) ?? '',
      userId: _str(j['user_id']),
      balance: _int(j['balance']),
      status: _str(j['status']),
      walletType: _str(j['wallet_type']),
      updatedAt: _date(j['updated_at']),
    );
  }

  Wallet copyWith({int? balance}) {
    return Wallet(
      walletId: walletId,
      userId: userId,
      balance: balance ?? this.balance,
      status: status,
      walletType: walletType,
      updatedAt: updatedAt,
    );
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
  static DateTime? _date(Object? v) => v is String ? DateTime.tryParse(v) : null;
}

/// Satu baris ledger_entries dari GET /wallets/{id}/history. JSON dari
/// internal/wallet/handler.go ledgerEntryResponse (field align
/// API_CONTRACT §6.5, tanpa wallet_id karena konteksnya per-wallet).
class LedgerEntry {
  const LedgerEntry({
    required this.ledgerId,
    required this.entryType,
    this.amount = 0,
    this.balanceAfter = 0,
    this.referenceType = '',
    this.referenceId = '',
    this.description = '',
    this.isReversed = false,
    this.createdAt,
  });

  final String ledgerId;
  final String entryType;
  final int amount;
  final int balanceAfter;
  final String referenceType;
  final String referenceId;
  final String description;
  final bool isReversed;
  final DateTime? createdAt;

  factory LedgerEntry.fromJson(Map<String, dynamic> j) {
    return LedgerEntry(
      ledgerId: _str(j['ledger_id']) ?? '',
      entryType: _str(j['entry_type']) ?? '',
      amount: _int(j['amount']),
      balanceAfter: _int(j['balance_after']),
      referenceType: _str(j['reference_type']) ?? '',
      referenceId: _str(j['reference_id']) ?? '',
      description: _str(j['description']) ?? '',
      isReversed: j['is_reversed'] == true,
      createdAt: _date(j['created_at']),
    );
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
  static DateTime? _date(Object? v) => v is String ? DateTime.tryParse(v) : null;
}

/// Hasil POST /wallets/{id}/topup (internal/wallet/service.go TopUpResponse).
class WalletTopUpResult {
  const WalletTopUpResult({
    required this.transactionId,
    required this.status,
    this.walletId = '',
    this.amount = 0,
    this.debtSettled,
    this.walletCredited,
    this.newBalance,
  });

  final String transactionId;
  final String walletId;
  final int amount;
  final int? debtSettled;
  final int? walletCredited;
  final int? newBalance;
  final String status;

  factory WalletTopUpResult.fromJson(Map<String, dynamic> j) {
    return WalletTopUpResult(
      transactionId: _str(j['transaction_id']) ?? '',
      walletId: _str(j['wallet_id']) ?? '',
      amount: _int(j['amount']),
      debtSettled: _intNullable(j['debt_settled']),
      walletCredited: _intNullable(j['wallet_credited']),
      newBalance: _intNullable(j['new_balance']),
      status: _str(j['status']) ?? 'PENDING',
    );
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
  static int? _intNullable(Object? v) {
    if (v is num) return v.round();
    return v == null || '$v'.isEmpty ? null : int.tryParse('$v');
  }
}

/// Hasil POST /wallets/{id}/transfer (internal/wallet/service.go
/// TransferResponse).
class WalletTransferResult {
  const WalletTransferResult({
    required this.transferId,
    this.fromBalance = 0,
    this.toBalance = 0,
  });

  final String transferId;
  final int fromBalance;
  final int toBalance;

  factory WalletTransferResult.fromJson(Map<String, dynamic> j) {
    return WalletTransferResult(
      transferId: _str(j['transfer_id']) ?? '',
      fromBalance: _int(j['from_balance']),
      toBalance: _int(j['to_balance']),
    );
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
}