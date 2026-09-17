import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/models/driver_order.dart';
import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/providers/order_provider.dart';
import 'package:driver_app/screens/multi_stop_delivery_screen.dart';

import 'test_helpers.dart';

Widget _build(DriverOrder? order, {List<DriverOrder> active = const []}) {
  return ProviderScope(
    overrides: [
      orderServiceProvider.overrideWithValue(FakeOrderService()),
      activeOrdersProvider.overrideWith((ref) => ActiveOrdersNotifier(FakeOrderService())..state = OrdersListState(orders: active)),
    ],
    child: MaterialApp(home: MultiStopDeliveryScreen(orderId: order?.id ?? 'x', initialOrder: order)),
  );
}

void main() {
  testWidgets('renders multi-stop send order with stops sorted', (tester) async {
    final order = makeSend('s-1');
    await tester.pumpWidget(_build(order));
    await tester.pump();
    await tester.pump();

    expect(find.textContaining('Send #S-1'), findsOneWidget);
    expect(find.text('0/2 selesai'), findsOneWidget);
    expect(find.text('Stop 1'), findsOneWidget);
    expect(find.text('Stop 2'), findsOneWidget);
    expect(find.text('Budi'), findsOneWidget);
    expect(find.text('Sari'), findsOneWidget);
    expect(find.textContaining('Jl. S1'), findsOneWidget);
    expect(find.textContaining('Jl. S2'), findsOneWidget);
    expect(find.text('Sudah Sampai'), findsNWidgets(1));
    expect(find.text('Navigasi'), findsNWidgets(1));
  });

  testWidgets('shows empty state when order has no stops', (tester) async {
    await tester.pumpWidget(_build(makeRide('r-1')));
    await tester.pump();
    expect(find.textContaining('belum memiliki stop'), findsOneWidget);
  });

  testWidgets('shows empty state when order is null', (tester) async {
    await tester.pumpWidget(_build(null));
    await tester.pump();
    expect(find.text('Multi-Stop'), findsOneWidget);
  });

  testWidgets('shows completed progress for partially delivered order', (tester) async {
    final order = makeSend('s-1');
    final advanced = order.copyWith(
      stops: [
        DriverStopCopier.copy(order.stops[0], status: 'COMPLETED'),
        order.stops[1],
      ],
    );
    await tester.pumpWidget(_build(advanced));
    await tester.pump();
    await tester.pump();

    expect(find.text('1/2 selesai'), findsOneWidget);
    expect(find.text('Sudah Sampai'), findsNWidgets(1));
  });
}
