import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/driver_earning.dart';
import '../services/earnings_service.dart';
import 'auth_provider.dart';

class EarningsState {
  const EarningsState({
    this.earning,
    this.isLoading = false,
    this.error,
  });

  final DriverEarning? earning;
  final bool isLoading;
  final String? error;

  EarningsState copyWith({
    DriverEarning? earning,
    bool? isLoading,
    String? error,
    bool clearError = false,
  }) {
    return EarningsState(
      earning: earning ?? this.earning,
      isLoading: isLoading ?? this.isLoading,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

final earningsProvider = StateNotifierProvider<EarningsNotifier, EarningsState>(
    (ref) => EarningsNotifier(ref.watch(earningsServiceProvider)));

class EarningsNotifier extends StateNotifier<EarningsState> {
  EarningsNotifier(this.service) : super(const EarningsState());

  final EarningsService service;

  Future<void> fetch(String driverId) async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final earning = await service.fetchEarnings(driverId);
      state = EarningsState(earning: earning);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: '$e', clearError: false);
    }
  }
}