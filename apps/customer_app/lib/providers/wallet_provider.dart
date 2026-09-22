import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/wallet.dart';
import '../services/wallet_service.dart';
import 'auth_provider.dart';

/// Provider layanan wallet. Memakai ApiClient yang sudah ada (Dio + JWT).
final walletServiceProvider = Provider((ref) => WalletService(ref.watch(apiClientProvider)));

/// State saldo wallet.
class WalletBalanceState {
  const WalletBalanceState({
    this.isLoading = false,
    this.error,
    this.wallet,
  });

  final bool isLoading;
  final String? error;
  final Wallet? wallet;

  WalletBalanceState copyWith({
    bool? isLoading,
    String? error,
    Wallet? wallet,
  }) {
    return WalletBalanceState(
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      wallet: wallet ?? this.wallet,
    );
  }
}

/// Notifier saldo wallet per wallet_id (family). Load otomatis saat pertama
/// di-watch; topUp/transfer memanggil ulang balance agar UI tetap segar.
class WalletNotifier extends StateNotifier<WalletBalanceState> {
  WalletNotifier(this._service, this._walletId) : super(const WalletBalanceState());

  final WalletService _service;
  final String _walletId;

  Future<void> load() async {
    state = const WalletBalanceState(isLoading: true);
    try {
      final wallet = await _service.getBalance(_walletId);
      state = WalletBalanceState(isLoading: false, wallet: wallet);
    } catch (_) {
      state = WalletBalanceState(
        isLoading: false,
        error: 'Gagal memuat saldo wallet. Pastikan server aktif.',
      );
    }
  }

  /// Top-up via service lalu refresh saldo. Mengembalikan true jika sukses.
  Future<bool> topUp(int amount) async {
    try {
      await _service.topUp(walletId: _walletId, amount: amount);
      await load();
      return true;
    } catch (_) {
      state = state.copyWith(
        error: 'Gagal melakukan top-up. Periksa koneksi lalu coba lagi.',
      );
      return false;
    }
  }

  /// Transfer P2P via service lalu refresh saldo. Mengembalikan true jika sukses.
  Future<bool> transfer({
    required String toWalletId,
    required int amount,
    String description = '',
  }) async {
    try {
      await _service.transfer(
        walletId: _walletId,
        toWalletId: toWalletId,
        amount: amount,
        description: description,
      );
      await load();
      return true;
    } catch (_) {
      state = state.copyWith(
        error: 'Gagal melakukan transfer. Periksa koneksi lalu coba lagi.',
      );
      return false;
    }
  }
}

final walletBalanceProvider =
    StateNotifierProvider.family<WalletNotifier, WalletBalanceState, String>((ref, walletId) {
  final notifier = WalletNotifier(ref.watch(walletServiceProvider), walletId);
  Future.microtask(notifier.load);
  return notifier;
});

/// Notifier wallet milik user terautentikasi (TD-120: GET /wallets/me).
/// Client tidak perlu tahu wallet_id.
class MyWalletNotifier extends StateNotifier<WalletBalanceState> {
  MyWalletNotifier(this._service, {this.walletType = 'CUSTOMER'})
      : super(const WalletBalanceState());

  final WalletService _service;
  final String walletType;

  Future<void> load() async {
    state = const WalletBalanceState(isLoading: true);
    try {
      final wallet = await _service.getMyWallet(type: walletType);
      state = WalletBalanceState(isLoading: false, wallet: wallet);
    } catch (_) {
      state = WalletBalanceState(
        isLoading: false,
        error: 'Gagal memuat saldo wallet. Pastikan server aktif.',
      );
    }
  }
}

/// Provider auto-resolve wallet milik user (tanpa argumen). Load otomatis
/// saat pertama di-watch.
final myWalletProvider =
    StateNotifierProvider<MyWalletNotifier, WalletBalanceState>((ref) {
  final notifier = MyWalletNotifier(ref.watch(walletServiceProvider));
  Future.microtask(notifier.load);
  return notifier;
});

/// Argumen riwayat wallet: id wallet + filter reference_type opsional.
class WalletHistoryArgs {
  const WalletHistoryArgs({required this.walletId, this.referenceType});

  final String walletId;
  final String? referenceType;
}

/// State riwayat transaksi wallet (pagination, mirror pola
/// FoodHistoryNotifier).
class WalletHistoryState {
  const WalletHistoryState({
    this.isLoading = false,
    this.error,
    this.entries = const [],
    this.hasMore = false,
  });

  final bool isLoading;
  final String? error;
  final List<LedgerEntry> entries;
  final bool hasMore;

  WalletHistoryState copyWith({
    bool? isLoading,
    String? error,
    List<LedgerEntry>? entries,
    bool? hasMore,
  }) {
    return WalletHistoryState(
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      entries: entries ?? this.entries,
      hasMore: hasMore ?? this.hasMore,
    );
  }
}

class WalletHistoryNotifier extends StateNotifier<WalletHistoryState> {
  WalletHistoryNotifier(this._service, this._args)
      : super(const WalletHistoryState());

  final WalletService _service;
  final WalletHistoryArgs _args;
  int _page = 1;

  static const int _pageSize = 20;

  Future<void> loadFirst() async {
    _page = 1;
    state = state.copyWith(isLoading: true, error: null);
    try {
      final result = await _service.getHistory(
        walletId: _args.walletId,
        page: _page,
        pageSize: _pageSize,
        referenceType: _args.referenceType,
      );
      state = WalletHistoryState(
        isLoading: false,
        entries: result.entries,
        hasMore: result.totalPages > _page,
      );
    } catch (_) {
      state = state.copyWith(
        isLoading: false,
        error: 'Gagal memuat riwayat transaksi.',
      );
    }
  }

  Future<void> loadMore() async {
    if (state.isLoading || !state.hasMore) return;
    _page++;
    state = state.copyWith(isLoading: true, error: null);
    try {
      final result = await _service.getHistory(
        walletId: _args.walletId,
        page: _page,
        pageSize: _pageSize,
        referenceType: _args.referenceType,
      );
      state = WalletHistoryState(
        isLoading: false,
        entries: [...state.entries, ...result.entries],
        hasMore: result.totalPages > _page,
      );
    } catch (_) {
      state = state.copyWith(isLoading: false, error: 'Gagal memuat halaman berikut.');
    }
  }
}

final walletHistoryProvider =
    StateNotifierProvider.family<WalletHistoryNotifier, WalletHistoryState, WalletHistoryArgs>(
  (ref, args) {
    return WalletHistoryNotifier(ref.watch(walletServiceProvider), args);
  },
);