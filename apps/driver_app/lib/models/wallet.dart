/// Model wallet (PayPulse/F001) — minimal, mirror customer_app.
///
/// Hanya field yang dipakai dashboard driver (saldo dari GET /wallets/me).
class Wallet {
  const Wallet({
    required this.walletId,
    this.userId,
    this.balance = 0,
    this.status,
    this.walletType,
  });

  final String walletId;
  final String? userId;
  final int balance;
  final String? status;
  final String? walletType;

  factory Wallet.fromJson(Map<String, dynamic> j) {
    return Wallet(
      walletId: _str(j['wallet_id']) ?? _str(j['id']) ?? '',
      userId: _str(j['user_id']),
      balance: _int(j['balance']),
      status: _str(j['status']),
      walletType: _str(j['wallet_type']),
    );
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();

  static int _int(Object? v) {
    if (v is num) return v.round();
    final parsed = double.tryParse('$v');
    return parsed?.round() ?? 0;
  }
}