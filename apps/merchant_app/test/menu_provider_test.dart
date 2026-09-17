import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:merchant_app/models/merchant_item.dart';
import 'package:merchant_app/models/merchant_menu.dart';
import 'package:merchant_app/models/merchant_order.dart';
import 'package:merchant_app/providers/auth_provider.dart';
import 'package:merchant_app/providers/menu_provider.dart';
import 'package:merchant_app/providers/analytics_provider.dart';
import 'package:merchant_app/providers/order_provider.dart';
import 'package:merchant_app/services/api_client.dart';
import 'package:merchant_app/services/menu_service.dart';
import 'package:merchant_app/services/order_service.dart';

class FakeDioAdapter extends HttpClientAdapter {
  final List<Map<String, dynamic>> responses;
  final List<Object> errors;
  int callCount = 0;

  FakeDioAdapter({this.responses = const [], this.errors = const []});

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    final idx = callCount++;
    if (idx < errors.length) {
      final e = errors[idx];
      if (e is DioException) throw e;
      throw e;
    }
    final r = responses[idx];
    return ResponseBody.fromString(_encode(r['body'] ?? {}), r['status'] ?? 200, headers: const {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}

String _encode(Object? o) {
  if (o is String) return o;
  if (o is Map) return '{${o.entries.map((e) => '"${e.key}":${_encode(e.value)}').join(',')}}';
  if (o is List) return '[${o.map(_encode).join(',')}]';
  if (o is bool) return o.toString();
  if (o is num) return o.toString();
  return 'null';
}

class FakeStorage extends Fake implements FlutterSecureStorage {
  final Map<String, String> store = {};
  @override
  Future<String?> read({required String key, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store[key];
  @override
  Future<void> write({required String key, required String? value, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store[key] = value ?? '';
  @override
  Future<void> delete({required String key, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store.remove(key);
}

ApiClient _api(FakeDioAdapter adapter, FakeStorage storage) {
  final dio = Dio(BaseOptions(baseUrl: 'http://test'));
  dio.httpClientAdapter = adapter;
  return ApiClient(dio: dio, storage: storage);
}

void main() {
  late FakeStorage storage;
  late ProviderContainer container;

  tearDown(() {
    container?.dispose();
  });

  ProviderContainer menuContainer(FakeDioAdapter adapter) {
    storage = FakeStorage()..store['merchant_id'] = 'm1';
    final api = _api(adapter, storage);
    return ProviderContainer(overrides: [
      storageProvider.overrideWithValue(storage),
      apiClientProvider.overrideWithValue(api),
      menuServiceProvider.overrideWithValue(MenuService(api)),
    ]);
  }

  group('MenuNotifier', () {
    test('load populates menus and items', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'menus': [{'menu_id': 'm1', 'name': 'Makanan', 'is_active': true}]}}},
        {'body': {'data': {'items': [
          {'item_id': 'i1', 'name': 'Nasi', 'price': 10000, 'menu_id': 'm1', 'stock': 5},
          {'item_id': 'i2', 'name': 'Es', 'price': 5000, 'menu_id': 'other', 'stock': 0},
        ]}}},
      ]);
      container = menuContainer(adapter);
      final notifier = container.read(menuProvider.notifier);
      await notifier.load();

      expect(notifier.state.menus.length, 1);
      expect(notifier.state.items.length, 2);
      final menuItems = notifier.itemsForMenu('m1');
      expect(menuItems.length, 1);
      expect(menuItems.single.name, 'Nasi');
    });

    test('load returns early when no merchant id', () async {
      storage = FakeStorage();
      container = ProviderContainer(overrides: [storageProvider.overrideWithValue(storage)]);
      final notifier = container.read(menuProvider.notifier);
      await notifier.load();
      expect(notifier.state.error, isNotNull);
    });

    test('load sets error on failure', () async {
      final adapter = FakeDioAdapter(errors: [DioException(requestOptions: RequestOptions(path: '/x'), message: 'fail')]);
      container = menuContainer(adapter);
      final notifier = container.read(menuProvider.notifier);
      await notifier.load();
      expect(notifier.state.error, isNotNull);
      expect(notifier.state.isLoading, isFalse);
    });

    test('createMenu then flow', () async {
      // createMenu call, then load() (2 calls)
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'id': 'm1', 'name': 'New'}}},
        {'body': {'data': {'menus': [{'menu_id': 'm1', 'name': 'New'}]}}},
        {'body': {'data': {'items': []}}},
      ]);
      container = menuContainer(adapter);
      final notifier = container.read(menuProvider.notifier);

      final ok = await notifier.createMenu(name: 'New', description: 'd');

      expect(ok, isTrue);
      expect(notifier.state.isMutating, isFalse);
      expect(notifier.state.menus.single.name, 'New');
    });

    test('createMenu fails without merchant id', () async {
      storage = FakeStorage();
      container = ProviderContainer(overrides: [storageProvider.overrideWithValue(storage)]);
      final notifier = container.read(menuProvider.notifier);
      final ok = await notifier.createMenu(name: 'New');
      expect(ok, isFalse);
    });

    test('createMenu error path', () async {
      final adapter = FakeDioAdapter(errors: [StateError('boom')]);
      container = menuContainer(adapter);
      final notifier = container.read(menuProvider.notifier);
      final ok = await notifier.createMenu(name: 'New');
      expect(ok, isFalse);
      expect(notifier.state.error, isNotNull);
      expect(notifier.state.isMutating, isFalse);
    });

    test('updateMenu', () async {
      // updateMenu call + load (2 calls)
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'id': 'm1', 'name': 'Upd'}}},
        {'body': {'data': {'menus': [{'menu_id': 'm1', 'name': 'Upd'}]}}},
        {'body': {'data': {'items': []}}},
      ]);
      container = menuContainer(adapter);
      final notifier = container.read(menuProvider.notifier);
      final ok = await notifier.updateMenu('m1', name: 'Upd', isActive: false);
      expect(ok, isTrue);
    });

    test('deleteMenu', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'ok': true}},
        {'body': {'data': {'menus': []}}},
        {'body': {'data': {'items': []}}},
      ]);
      container = menuContainer(adapter);
      final notifier = container.read(menuProvider.notifier);
      final ok = await notifier.deleteMenu('m1');
      expect(ok, isTrue);
      expect(notifier.state.menus, isEmpty);
    });

    test('createItem', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'item_id': 'i1', 'name': 'Item', 'price': 1000}}},
        {'body': {'data': {'menus': [{'menu_id': 'm1', 'name': 'Makanan'}]}}},
        {'body': {'data': {'items': [{'item_id': 'i1', 'name': 'Item', 'price': 1000, 'stock': 5, 'menu_id': 'm1', 'is_available': true}]}}},
      ]);
      container = menuContainer(adapter);
      final notifier = container.read(menuProvider.notifier);
      final ok = await notifier.createItem(menuId: 'm1', name: 'Item', price: 1000, stock: 5, isAvailable: true);
      expect(ok, isTrue);
      expect(notifier.state.items.single.name, 'Item');
    });

    test('createItem fails without merchant id', () async {
      storage = FakeStorage();
      container = ProviderContainer(overrides: [storageProvider.overrideWithValue(storage)]);
      final notifier = container.read(menuProvider.notifier);
      final ok = await notifier.createItem(menuId: 'm1', name: 'Item');
      expect(ok, isFalse);
    });

    test('updateItem', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'item_id': 'i1', 'name': 'Upd'}}},
        {'body': {'data': {'menus': [{'menu_id': 'm1', 'name': 'M'}]}}},
        {'body': {'data': {'items': [{'item_id': 'i1', 'name': 'Upd', 'stock': 2, 'menu_id': 'm1'}]}}},
      ]);
      container = menuContainer(adapter);
      final notifier = container.read(menuProvider.notifier);
      final ok = await notifier.updateItem('i1', name: 'Upd', stock: 2);
      expect(ok, isTrue);
    });

    test('deleteItem', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'ok': true}},
        {'body': {'data': {'menus': [{'menu_id': 'm1', 'name': 'M'}]}}},
        {'body': {'data': {'items': []}}},
      ]);
      container = menuContainer(adapter);
      final notifier = container.read(menuProvider.notifier);
      final ok = await notifier.deleteItem('i1');
      expect(ok, isTrue);
      expect(notifier.state.items, isEmpty);
    });

    test('MenuState copyWith', () {
      const s = MenuState();
      final c = s.copyWith(menus: [], items: [], isLoading: true, error: 'e', isMutating: true);
      expect(c.isLoading, isTrue);
      expect(c.isMutating, isTrue);
      expect(c.error, 'e');
    });
  });

  group('analyticsProvider', () {
    test('computes summary from orders in state', () async {
      storage = FakeStorage()..store['merchant_id'] = 'm1';
      final now = DateTime.now();
      final today = DateTime(now.year, now.month, now.day);
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'orders': [
          {
            'id': 'a', 'status': 'DELIVERED', 'merchant_status': 'READY', 'created_at': today.toIso8601String(), 'total_amount': 20000,
            'items': [{'item_name': 'Nasi', 'quantity': 2}],
          },
        ]}}},
      ]);
      final api = _api(adapter, storage);
      container = ProviderContainer(overrides: [
        storageProvider.overrideWithValue(storage),
        apiClientProvider.overrideWithValue(api),
        orderServiceProvider.overrideWithValue(OrderService(api)),
      ]);
      await container.read(orderProvider.notifier).load();

      final summary = container.read(analyticsProvider('today'));
      expect(summary.totalOrders, 1);
      expect(summary.totalRevenue, 20000);
      expect(summary.topItems.single.quantity, 2);
    });
  });
}
