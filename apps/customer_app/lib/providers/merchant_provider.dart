import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/merchant.dart';
import '../services/merchant_service.dart';
import 'auth_provider.dart';

final merchantServiceProvider = Provider((ref) => MerchantService(ref.watch(apiClientProvider)));

/// State daftar merchant (food catalog).
class MerchantListState {
  const MerchantListState({
    this.isLoading = false,
    this.error,
    this.merchants = const [],
    this.query = '',
    this.pendingMerchants = const [],
  });

  final bool isLoading;
  final String? error;
  final List<Merchant> merchants;
  final String query;
  final List<Merchant> pendingMerchants;

  MerchantListState copyWith({
    bool? isLoading,
    String? error,
    List<Merchant>? merchants,
    String? query,
    List<Merchant>? pendingMerchants,
  }) {
    return MerchantListState(
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      merchants: merchants ?? this.merchants,
      query: query ?? this.query,
      pendingMerchants: pendingMerchants ?? this.pendingMerchants,
    );
  }

  /// Hasil setelah difilter oleh query pencarian (search client-side).
  List<Merchant> get visible {
    final q = query.trim().toLowerCase();
    if (q.isEmpty) return merchants;
    return merchants.where((m) {
      return m.name.toLowerCase().contains(q) ||
          m.category.toLowerCase().contains(q);
    }).toList();
  }
}

class MerchantListNotifier extends StateNotifier<MerchantListState> {
  MerchantListNotifier(this._service) : super(const MerchantListState());

  final MerchantService _service;

  Future<void> load({double? lat, double? lng}) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final merchants = await _service.fetchMerchants(lat: lat, lng: lng);
      state = state.copyWith(isLoading: false, merchants: merchants);
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: 'Gagal memuat daftar merchant. Pastikan koneksi & server aktif.',
      );
    }
  }

  void setQuery(String query) => state = state.copyWith(query: query);

  /// Urutkan menurut jarak (populasi contoh demo bila backend tak ada jarak).
  void sortByDistance() {
    final sorted = [...state.merchants]
      ..sort((a, b) => a.distanceKm.compareTo(b.distanceKm));
    state = state.copyWith(merchants: sorted);
  }

  /// Urutkan menurut rating tertinggi.
  void sortByRating() {
    final sorted = [...state.merchants]
      ..sort((a, b) => b.rating.compareTo(a.rating));
    state = state.copyWith(merchants: sorted);
  }
}

final merchantListProvider =
    StateNotifierProvider<MerchantListNotifier, MerchantListState>((ref) {
  return MerchantListNotifier(ref.watch(merchantServiceProvider));
});
