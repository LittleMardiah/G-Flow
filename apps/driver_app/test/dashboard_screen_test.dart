import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/models/wallet.dart';
import 'package:driver_app/providers/auth_provider.dart';
import 'package:driver_app/providers/order_provider.dart';
import 'package:driver_app/providers/wallet_provider.dart';
import 'package:driver_app/screens/dashboard_screen.dart';
import 'package:driver_app/services/api_client.dart';
import 'package:driver_app/services/location_service.dart';
import 'package:driver_app/services/wallet_service.dart';

import 'test_helpers.dart';

class FakeHttpOverrides extends HttpOverrides {}

class FakeWalletService extends WalletService {
  FakeWalletService({this.wallet}) : super(ApiClient());

  final Wallet? wallet;

  @override
  Future<Wallet> getMyWallet({String type = 'DRIVER'}) async =>
      wallet ?? const Wallet(walletId: 'w1', balance: 1500000, status: 'ACTIVE');
}

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
      walletServiceProvider.overrideWithValue(FakeWalletService()),
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
    expect(find.text('Kapasitas: 0/3'), findsOneWidget);

    // Konten di bawah map + kartu wallet berada di luar viewport default
    // → scroll agar item di-build oleh ListView.
    await tester.scrollUntilVisible(find.text('Order Aktif'), 200,
        scrollable: find.byType(Scrollable).first);
    expect(find.text('Order Aktif'), findsOneWidget);
    expect(find.text('Lihat pesanan tersedia'), findsOneWidget);
    await tester.scrollUntilVisible(find.textContaining('Belum ada order aktif'), 200,
        scrollable: find.byType(Scrollable).first);
    expect(find.textContaining('Belum ada order aktif'), findsOneWidget);
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

    // Scroll the ListView to render remaining order cards.
    await tester.scrollUntilVisible(find.textContaining('#R-1'), 200,
        scrollable: find.byType(Scrollable).first);
    expect(find.textContaining('#R-1'), findsOneWidget);
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

  testWidgets('renders wallet balance card from GET /wallets/me', (tester) async {
    await tester.pumpWidget(_build(driverId: 'd-1'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.text('Saldo Wallet'), findsOneWidget);
    expect(find.text('Rp 1.500.000'), findsOneWidget);
    expect(find.text('ACTIVE'), findsOneWidget);
  });

  testWidgets('wallet error shows graceful placeholder, dashboard still renders', (tester) async {
    final failing = WalletServiceFakeFails();
    await tester.pumpWidget(ProviderScope(
      overrides: [
        orderServiceProvider.overrideWithValue(FakeOrderService()),
        locationServiceProvider.overrideWithValue(LocationService(ApiClient())),
        currentDriverIdProvider.overrideWith((_) async => 'd-1'),
        activeOrdersProvider.overrideWith(
          (ref) => ActiveOrdersNotifier(FakeOrderService())
            ..state = const OrdersListState(orders: []),
        ),
        availableOrdersProvider.overrideWith((ref) => AvailableOrdersNotifier(FakeOrderService())),
        walletServiceProvider.overrideWithValue(failing),
      ],
      child: const MaterialApp(home: DashboardScreen()),
    ));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.text('Saldo Wallet'), findsOneWidget);
    expect(find.textContaining('Saldo tidak tersedia'), findsOneWidget);
    await tester.scrollUntilVisible(find.text('Order Aktif'), 200,
        scrollable: find.byType(Scrollable).first);
    expect(find.text('Order Aktif'), findsOneWidget);
  });
}

class WalletServiceFakeFails extends WalletService {
  WalletServiceFakeFails() : super(ApiClient());

  @override
  Future<Wallet> getMyWallet({String type = 'DRIVER'}) async =>
      throw StateError('WALLET_NOT_FOUND');
}
