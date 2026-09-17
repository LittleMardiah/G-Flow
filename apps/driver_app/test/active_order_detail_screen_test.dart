import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/models/driver_order.dart';
import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/providers/order_provider.dart';
import 'package:driver_app/screens/active_order_detail_screen.dart';

import 'test_helpers.dart';

Widget _build(DriverOrder? order) {
  final svc = FakeOrderService();
  return ProviderScope(
    overrides: [
      orderServiceProvider.overrideWithValue(svc),
      activeOrdersProvider.overrideWith((ref) => ActiveOrdersNotifier(svc)),
      availableOrdersProvider.overrideWith((ref) => AvailableOrdersNotifier(svc)),
    ],
    child: MaterialApp(home: ActiveOrderDetailScreen(orderId: order?.id ?? 'x', initialOrder: order)),
  );
}

Future<void> _pumpMap(WidgetTester tester) async {
  await tester.pump();
  await tester.pump();
  await drainMapExceptions(tester);
}

void main() {
  testWidgets('renders ride order detail with next action', (tester) async {
    final order = makeRide('r-1', status: 'DRIVER_ASSIGNED');
    await tester.pumpWidget(_build(order));
    await _pumpMap(tester);

    expect(find.textContaining('Ride #R-1'), findsOneWidget);
    expect(find.text('Tiba di titik pickup'), findsOneWidget);
    expect(find.textContaining('Earning'), findsOneWidget);
  });

  testWidgets('ride TRIP_STARTED shows complete action', (tester) async {
    final order = makeRide('r-1', status: 'TRIP_STARTED');
    await tester.pumpWidget(_build(order));
    await _pumpMap(tester);
    expect(find.text('Selesaikan perjalanan'), findsOneWidget);
  });

  testWidgets('food order shows merchant and pickup action', (tester) async {
    final order = makeFood('f-1');
    await tester.pumpWidget(_build(order));
    await _pumpMap(tester);
    expect(find.textContaining('Food #F-1'), findsOneWidget);
    expect(find.text('Ambil pesanan dari merchant'), findsOneWidget);
    expect(find.textContaining('Merchant'), findsOneWidget);
  });

  testWidgets('send order IN_TRANSIT shows multi-stop button', (tester) async {
    final order = makeSend('s-1');
    await tester.pumpWidget(_build(order));
    await _pumpMap(tester);
    expect(find.textContaining('Send #S-1'), findsOneWidget);
    expect(find.text('Buka Multi-Stop Delivery'), findsOneWidget);
    expect(find.textContaining('Stops (0/2)'), findsOneWidget);
  });

  testWidgets('shows terminal message for completed order', (tester) async {
    final order = makeRide('r-1', status: 'COMPLETED');
    await tester.pumpWidget(_build(order));
    await _pumpMap(tester);
    expect(find.textContaining('sudah selesai'), findsOneWidget);
  });

  testWidgets('shows not-found empty state when order is null', (tester) async {
    await tester.pumpWidget(_build(null));
    await tester.pump();
    expect(find.textContaining('Order tidak ditemukan'), findsOneWidget);
  });
}
