import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/models/driver_earning.dart';
import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/providers/earnings_provider.dart';
import 'package:driver_app/services/api_client.dart';
import 'package:driver_app/services/earnings_service.dart';

class _FakeEarningsService extends EarningsService {
  _FakeEarningsService() : super(ApiClient());
  bool fail = false;

  @override
  Future<DriverEarning> fetchEarnings(String driverId) async {
    if (fail) throw Exception('earnings fail');
    return const DriverEarning(todayTotal: 100, todayOrderCount: 5);
  }
}

void main() {
  test('initial state is empty, not loading', () {
    final container = ProviderContainer(overrides: [
      earningsServiceProvider.overrideWithValue(_FakeEarningsService()),
    ]);
    addTearDown(container.dispose);
    final state = container.read(earningsProvider);
    expect(state.isLoading, isFalse);
    expect(state.earning, isNull);
    expect(state.error, isNull);
  });

  test('fetch populates earning and clears loading/error', () async {
    final container = ProviderContainer(overrides: [
      earningsServiceProvider.overrideWithValue(_FakeEarningsService()),
    ]);
    addTearDown(container.dispose);
    final notifier = container.read(earningsProvider.notifier);
    await notifier.fetch('driver-1');
    final state = container.read(earningsProvider);
    expect(state.isLoading, isFalse);
    expect(state.error, isNull);
    expect(state.earning?.todayTotal, 100);
    expect(state.earning?.todayOrderCount, 5);
  });

  test('fetch sets error on failure', () async {
    final service = _FakeEarningsService()..fail = true;
    final container = ProviderContainer(overrides: [
      earningsServiceProvider.overrideWithValue(service),
    ]);
    addTearDown(container.dispose);
    final notifier = container.read(earningsProvider.notifier);
    await notifier.fetch('driver-1');
    final state = container.read(earningsProvider);
    expect(state.isLoading, isFalse);
    expect(state.earning, isNull);
    expect(state.error, contains('earnings fail'));
  });

  test('copyWith clearError works', () {
    const s = EarningsState(error: 'err');
    final cleared = s.copyWith(clearError: true);
    expect(cleared.error, isNull);
    final kept = s.copyWith();
    expect(kept.error, 'err');
  });
}
