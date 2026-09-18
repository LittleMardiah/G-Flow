import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/providers/order_provider.dart';
import 'package:driver_app/screens/dashboard_screen.dart';
import 'package:driver_app/services/api_client.dart';
import 'package:driver_app/services/location_service.dart';

import 'test_helpers.dart';

class FakeHttpOverrides extends HttpOverrides {}

Widget _build({
  FakeOrderService? service,
  String? driverId = 'driver-1',
  OrdersListState? activeState,
}) {
  final svc = service ?? FakeOrderService();
  return ProviderScope(
    overrides: [
      orderServiceProvider.overrideWithValue(svc),
      locationServiceProvider.overrideWithValue(LocationService(ApiClient())),
      currentDriverIdProvider.overrideWith((_) async => driverId),
      activeOrdersProvider.overrideWith(
        (ref) => ActiveOrdersNotifier(svc)
          ..state = activeState ?? OrdersListState(orders: svc.activeOrders),
      ),
      availableOrdersProvider.overrideWith((ref) => AvailableOrdersNotifier(svc)),
    ],
    child: const MaterialApp(home: DashboardScreen()),
  );
}

void main() {
  setUp(() {
    HttpOverrides.global = FakeHttpOverrides();
  });

  testWidgets('renders header, empty state and capacity card', (tester) async {
    await tester.pumpWidget(_build(driverId: null));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    expect(find.text('G-Flow Driver'), findsOneWidget);
    expect(find.text('ONLINE'), findsOneWidget);
    expect(find.textContaining('Belum ada order aktif'), findsOneWidget);
    expect(find.text('Kapasitas: 0/3'), findsOneWidget);
    expect(find.text('Order Aktif'), findsOneWidget);
    expect(find.text('Lihat pesanan tersedia'), findsOneWidget);
  });

  testWidgets('toggling offline shows OFFLINE', (tester) async {
    await tester.pumpWidget(_build(driverId: null));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    await tester.tap(find.byType(Switch));
    await tester.pump();
    expect(find.text('OFFLINE'), findsOneWidget);
  });

  testWidgets('renders active orders grouped by type', (tester) async {
    final svc = FakeOrderService();
    svc.activeOrders = [makeRide('r-1', status: 'TRIP_STARTED'), makeFood('f-1'), makeSend('s-1')];
    await tester.pumpWidget(_build(service: svc));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.text('Kapasitas: 3/3'), findsOneWidget);
    expect(find.text('Capacity Reached'), findsOneWidget);
    expect(find.textContaining('#R-1'), findsOneWidget);

    // Scroll the ListView to render remaining order cards.
    await tester.scrollUntilVisible(find.textContaining('#S-1'), 200,
        scrollable: find.byType(Scrollable).first);
    expect(find.textContaining('#F-1'), findsOneWidget);
    expect(find.textContaining('#S-1'), findsOneWidget);
  });

  testWidgets('shows error banner when active orders error', (tester) async {
    await tester.pumpWidget(_build(
      driverId: null,
      activeState: const OrdersListState(orders: [], error: 'Kapasitas penuh (maks 3 order aktif)'),
    ));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    expect(find.textContaining('Kapasitas penuh'), findsOneWidget);
  });
}
