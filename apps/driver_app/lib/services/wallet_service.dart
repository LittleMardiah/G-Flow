import '../models/wallet.dart';
import 'api_client.dart';

/// Layanan data wallet untuk driver_app (TD-074: saldo di dashboard).
///
/// Endpoint (source of truth: cmd/api/main.go + internal/wallet/handler.go):
/// - GET  /api/v1/wallets/me?type=DRIVER (TD-120, auto-resolve wallet_id).
class WalletService {
  const WalletService(this.apiClient);

  final ApiClient apiClient;

  /// GET /api/v1/wallets/me — auto-resolve wallet milik user terautentikasi
  /// (TD-120). Query `type` default DRIVER di sini. Response data:
  /// {wallet_id, balance, status, wallet_type}. 404 (WALLET_NOT_FOUND) jika
  /// user tidak punya wallet tipe tsb → dibiarkan throw agar UI tampil
  /// placeholder graceful.
  Future<Wallet> getMyWallet({String type = 'DRIVER'}) async {
    final res = await apiClient.get('/api/v1/wallets/me?type=$type');
    final data = res.data is Map<String, dynamic>
        ? (res.data as Map)['data']
        : null;
    if (data is! Map<String, dynamic>) {
      throw StateError('Respons tidak valid untuk wallet saya (type=$type)');
    }
    return Wallet.fromJson(data);
  }
}