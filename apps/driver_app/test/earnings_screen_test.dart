import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/models/driver_earning.dart';
import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/providers/earnings_provider.dart';
import 'package:driver_app/screens/earnings_screen.dart';
import 'package:driver_app/services/api_client.dart';
import 'package:driver_app/services/earnings_service.dart';

class _FakeEarningsService extends EarningsService {
  _FakeEarningsService() : super(ApiClient());
  bool fail = false;

  @override
  Future<DriverEarning> fetchEarnings(String driverId) async {
    if (fail) throw Exception('earnings fail');
    return const DriverEarning(
      todayTotal: 285000,
      todayOrderCount: 12,
      weekTotal: 1675000,
      rideCount: 6,
      foodCount: 5,
      sendCount: 1,
      daily: [EarningPoint(label: '09.00', amount: 12000), EarningPoint(label: '10.00', amount: 35000)],
      weekly: [EarningPoint(label: 'Sen', amount: 180000, orderCount: 8)],
      isMock: true,
    );
  }
}

Widget _build(EarningsService service) {
  return ProviderScope(
    overrides: [
      earningsServiceProvider.overrideWithValue(service),
      currentDriverIdProvider.overrideWith((_) async => 'driver-1'),
    ],
    child: const MaterialApp(home: EarningsScreen()),
  );
}

void main() {
  testWidgets('renders earnings data and chart', (tester) async {
    await tester.pumpWidget(_build(_FakeEarningsService()));
    await tester.pumpAndSettle();

    expect(find.text('Earnings'), findsOneWidget);
    expect(find.text('Rp 285.000'), findsOneWidget);
    expect(find.text('12'), findsOneWidget);
    expect(find.text('6'), findsOneWidget);
    expect(find.text('5'), findsOneWidget);
    expect(find.text('1'), findsOneWidget);
    expect(find.text('Ride'), findsOneWidget);
    expect(find.text('Food'), findsOneWidget);
    expect(find.text('Send'), findsOneWidget);
    expect(find.text('Harian'), findsOneWidget);
    expect(find.text('Mingguan'), findsOneWidget);
    expect(find.text('Tarik Pendapatan (Withdrawal)'), findsOneWidget);
    expect(find.text('DEMO'), findsOneWidget);
  });

  testWidgets('toggles to weekly chart', (tester) async {
    await tester.pumpWidget(_build(_FakeEarningsService()));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Mingguan'));
    await tester.pumpAndSettle();
    expect(find.text('Rp 1.675.000'), findsOneWidget);
  });

  testWidgets('withdraw button shows unavailable message', (tester) async {
    await tester.pumpWidget(_build(_FakeEarningsService()));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Tarik Pendapatan (Withdrawal)'));
    await tester.pump(const Duration(seconds: 1));
    await tester.pump();
    expect(find.textContaining('withdrawal belum tersedia'), findsOneWidget);
  });

  testWidgets('shows error state when fetch fails', (tester) async {
    final service = _FakeEarningsService()..fail = true;
    await tester.pumpWidget(_build(service));
    await tester.pumpAndSettle();
    expect(find.textContaining('Gagal memuat earnings'), findsOneWidget);
  });
}
