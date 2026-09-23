import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/wallet.dart';
import '../services/wallet_service.dart';
import 'auth_provider.dart';

/// Provider layanan wallet (Dio + JWT via ApiClient yang sudah ada).
final walletServiceProvider =
    Provider<WalletService>((ref) => WalletService(ref.watch(apiClientProvider)));

/// State saldo wallet milik user terautentikasi (mirror customer_app).
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

/// Notifier wallet auto-resolve via GET /wallets/me?type=DRIVER (TD-120).
class MyWalletNotifier extends StateNotifier<WalletBalanceState> {
  MyWalletNotifier(this._service, {this.walletType = 'DRIVER'})
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
        error: 'Saldo wallet tidak tersedia. Pastikan server aktif.',
      );
    }
  }
}

/// Provider wallet saldo driver. Load otomatis saat pertama di-watch;
/// dashboard memanggil `load()` ulang pada pull-to-refresh.
final myWalletProvider = StateNotifierProvider<MyWalletNotifier, WalletBalanceState>(
    (ref) {
  final notifier = MyWalletNotifier(ref.watch(walletServiceProvider));
  Future.microtask(notifier.load);
  return notifier;
});