import 'package:uuid/uuid.dart';

import '../models/wallet.dart';
import 'api_client.dart';

/// Layanan data wallet (PayPulse/F001) untuk customer_app.
///
/// Endpoint (main.go:196-201, KODE = source of truth):
/// - GET  /api/v1/wallets/me?type=CUSTOMER (TD-120, auto-resolve wallet_id)
/// - GET  /api/v1/wallets/{wallet_id}/balance
/// - GET  /api/v1/wallets/{wallet_id}/history?page=&page_size=&reference_type=
/// - POST /api/v1/wallets/{wallet_id}/topup
/// - POST /api/v1/wallets/{wallet_id}/transfer
class WalletService {
  const WalletService(this.apiClient);

  final ApiClient apiClient;

  /// GET /api/v1/wallets/me — auto-resolve wallet milik user terautentikasi
  /// (TD-120). Client TIDAK perlu tahu wallet_id. Query `type` default
  /// CUSTOMER di backend. Response data: {wallet_id, balance, status,
  /// wallet_type}. 404 jika user tidak punya wallet tipe tsb.
  Future<Wallet> getMyWallet({String type = 'CUSTOMER'}) async {
    final res = await apiClient.get('/api/v1/wallets/me?type=$type');
    final data = res.data is Map<String, dynamic>
        ? (res.data as Map)['data']
        : null;
    if (data is! Map<String, dynamic>) {
      throw StateError('Respons tidak valid untuk wallet saya (type=$type)');
    }
    return Wallet.fromJson(data);
  }

  /// GET /api/v1/wallets/{wallet_id}/balance — saldo wallet milik user.
  Future<Wallet> getBalance(String walletId) async {
    final res = await apiClient.get('/api/v1/wallets/$walletId/balance');
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (data is! Map<String, dynamic>) {
      throw StateError('Respons tidak valid untuk balance wallet $walletId');
    }
    return Wallet.fromJson(data);
  }

  /// GET /api/v1/wallets/{wallet_id}/history — riwayat ledger entries dengan
  /// pagination. Query: page (>=1), page_size (1..50), reference_type (opsional).
  /// Response: `{data:{entries:[...]}, meta:{page,page_size,total,total_pages}}`.
  Future<WalletHistoryPage> getHistory({
    required String walletId,
    int page = 1,
    int pageSize = 20,
    String? referenceType,
  }) async {
    final q = StringBuffer(
      '/api/v1/wallets/$walletId/history?page=$page&page_size=$pageSize',
    );
    if (referenceType != null && referenceType.trim().isNotEmpty) {
      q.write('&reference_type=${Uri.encodeQueryComponent(referenceType)}');
    }
    final res = await apiClient.get(q.toString());
    final body = res.data is Map<String, dynamic> ? res.data as Map : const <dynamic, dynamic>{};
    final data = body['data'] is Map ? body['data'] as Map : const <dynamic, dynamic>{};
    final meta = body['meta'] is Map ? body['meta'] as Map : const <dynamic, dynamic>{};
    final items = data['entries'] is List ? data['entries'] as List : const <dynamic>[];
    final entries = items
        .whereType<Map>()
        .map((e) => LedgerEntry.fromJson(e.cast<String, dynamic>()))
        .toList();
    return WalletHistoryPage(
      entries: entries,
      page: (meta['page'] is num ? meta['page'] as num : 1).round(),
      pageSize: (meta['page_size'] is num ? meta['page_size'] as num : 0).round(),
      total: (meta['total'] is num ? meta['total'] as num : 0).round(),
      totalPages: (meta['total_pages'] is num ? meta['total_pages'] as num : 0).round(),
    );
  }

  /// POST /api/v1/wallets/{wallet_id}/topup — top-up (payment SIMULATED).
  ///
  /// Idempotensi: backend wallet membaca `idempotency_key` dari JSON BODY
  /// (internal/wallet/handler.go topUpRequestBody), bukan dari header seperti
  /// ride/food (handler.go read c.GetHeader). Karena itu key dikirim di body;
  /// header X-Idempotency-Key ikut dikirim (kompatibel + interceptor
  /// ApiClient) sehingga aman bila backend nanti pindah ke header.
  Future<WalletTopUpResult> topUp({
    required String walletId,
    required int amount,
    String? idempotencyKey,
  }) async {
    final idem = idempotencyKey ?? const Uuid().v4();
    final res = await apiClient.post(
      '/api/v1/wallets/$walletId/topup',
      headers: {'X-Idempotency-Key': idem},
      data: {
        'amount': amount,
        'idempotency_key': idem,
      },
    );
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (data is! Map<String, dynamic>) {
      throw StateError('Respons tidak valid untuk top-up wallet $walletId');
    }
    return WalletTopUpResult.fromJson(data);
  }

  /// POST /api/v1/wallets/{wallet_id}/transfer — transfer P2P ke wallet lain.
  ///
  /// Idempotensi: sama seperti topUp, key dikirim di body (`idempotency_key`)
  /// sesuai transferRequestBody backend + header X-Idempotency-Key.
  Future<WalletTransferResult> transfer({
    required String walletId,
    required String toWalletId,
    required int amount,
    String description = '',
    String? idempotencyKey,
  }) async {
    final idem = idempotencyKey ?? const Uuid().v4();
    final res = await apiClient.post(
      '/api/v1/wallets/$walletId/transfer',
      headers: {'X-Idempotency-Key': idem},
      data: {
        'to_wallet_id': toWalletId,
        'amount': amount,
        'description': description,
        'idempotency_key': idem,
      },
    );
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (data is! Map<String, dynamic>) {
      throw StateError('Respons tidak valid untuk transfer wallet $walletId');
    }
    return WalletTransferResult.fromJson(data);
  }
}

/// Hasil GET /wallets/{id}/history — daftar entries + info pagination dari
/// meta (page, page_size, total, total_pages) untuk menentukan hasMore.
class WalletHistoryPage {
  const WalletHistoryPage({
    required this.entries,
    this.page = 1,
    this.pageSize = 20,
    this.total = 0,
    required this.totalPages,
  });

  final List<LedgerEntry> entries;
  final int page;
  final int pageSize;
  final int total;
  final int totalPages;

  bool get isEmpty => entries.isEmpty;
}