import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/config/constants.dart';
import 'package:driver_app/models/driver_order.dart';
import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/providers/order_provider.dart';
import 'package:driver_app/screens/available_orders_screen.dart';

import 'test_helpers.dart';

Widget _build(FakeOrderService service, {List<DriverOrder> active = const []}) {
  return ProviderScope(
    overrides: [
      orderServiceProvider.overrideWithValue(service),
      activeOrdersProvider.overrideWith(
        (ref) => ActiveOrdersNotifier(service)..state = OrdersListState(orders: active),
      ),
    ],
    child: const MaterialApp(home: AvailableOrdersScreen()),
  );
}

void main() {
  testWidgets('renders empty state when no available orders', (tester) async {
    final service = FakeOrderService();
    await tester.pumpWidget(_build(service));
    await tester.pump();
    await tester.pump();

    expect(find.text('Pesanan Tersedia'), findsOneWidget);
    expect(find.text('Tidak ada pesanan tersedia saat ini.'), findsOneWidget);
    expect(find.text('Semua'), findsOneWidget);
    expect(find.text('Ride'), findsOneWidget);
    expect(find.text('Food'), findsOneWidget);
    expect(find.text('Send'), findsOneWidget);
  });

  testWidgets('renders order cards with type-specific layout', (tester) async {
    final service = FakeOrderService()
      ..availableOrders = [makeRide('r-1'), makeFood('f-1'), makeSend('s-1')];
    await tester.pumpWidget(_build(service));
    await tester.pump();
    await tester.pump();

    expect(find.textContaining('#R-1'), findsOneWidget);
    expect(find.textContaining('#F-1'), findsOneWidget);
    expect(find.textContaining('#S-1'), findsOneWidget);
    expect(find.text('ACCEPT'), findsNWidgets(3));

    // Advance past countdown to cancel pending per-card timers.
    await tester.pump(const Duration(seconds: 31));
  });

  testWidgets('shows capacity reached banner when at capacity', (tester) async {
    final service = FakeOrderService()
      ..availableOrders = [makeRide('r-1')]
      ..activeOrders = List.generate(kMaxActiveOrders, (i) => makeRide('a-$i', status: 'TRIP_STARTED'));
    await tester.pumpWidget(
      _build(service, active: List.generate(kMaxActiveOrders, (i) => makeRide('a-$i', status: 'TRIP_STARTED'))),
    );
    await tester.pump();
    await tester.pump();

    expect(find.textContaining('Capacity Reached'), findsOneWidget);
    expect(find.text('Kapasitas penuh'), findsOneWidget);
    await tester.pump(const Duration(seconds: 31));
  });

  testWidgets('filter by type and sort options work', (tester) async {
    final service = FakeOrderService()
      ..availableOrders = [makeRide('r-1'), makeFood('f-1'), makeSend('s-1')];
    await tester.pumpWidget(_build(service));
    await tester.pump();
    await tester.pump();

    // Filter to Food only
    await tester.tap(find.widgetWithText(ChoiceChip, 'Food'));
    await tester.pump();
    expect(find.textContaining('#R-1'), findsNothing);
    expect(find.textContaining('#F-1'), findsOneWidget);
    expect(find.textContaining('#S-1'), findsNothing);

    // Open sort menu
    await tester.tap(find.byIcon(Icons.sort));
    await tester.pumpAndSettle();
    expect(find.text('Jarak terdekat'), findsOneWidget);
    expect(find.text('Tipe order'), findsOneWidget);
    expect(find.text('Waktu pickup'), findsOneWidget);
    await tester.tap(find.text('Tipe order'));
    await tester.pump();

    // Back to all
    await tester.tap(find.widgetWithText(ChoiceChip, 'Semua'));
    await tester.pump();
    expect(find.textContaining('#R-1'), findsOneWidget);

    await tester.pump(const Duration(seconds: 31));
  });

  testWidgets('accepting an order removes it from available', (tester) async {
    final service = FakeOrderService()
      ..availableOrders = [makeRide('r-1')];
    await tester.pumpWidget(_build(service));
    await tester.pump();
    await tester.pump();

    await tester.tap(find.text('ACCEPT'));
    await tester.pump();
    await tester.pump();

    expect(find.text('ACCEPT'), findsNothing);
    expect(find.textContaining('diterima'), findsOneWidget);
    await tester.pump(const Duration(seconds: 31));
  });
}
