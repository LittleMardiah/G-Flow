import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:merchant_app/services/api_client.dart';
import 'package:merchant_app/services/menu_service.dart';
import 'package:merchant_app/services/merchant_service.dart';
import 'package:merchant_app/services/order_service.dart';

class FakeDioAdapter implements HttpClientAdapter {
  final List<Map<String, dynamic>> responses;
  int callCount = 0;
  final List<RequestOptions> requests = [];

  FakeDioAdapter({required this.responses});

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    requests.add(options);
    final r = responses[callCount++];
    return ResponseBody.fromString(
      _encode(r['body'] ?? {}),
      r['status'] ?? 200,
      headers: const {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

String _encode(Object? data) {
  // Use simple JSON string encoding without dart:convert conflict.
  if (data is String) return data;
  return _enc(data);
}

String _enc(Object? o) {
  if (o is Map) {
    final parts = o.entries.map((e) => '"${e.key}":${_enc(e.value)}').join(',');
    return '{$parts}';
  }
  if (o is List) return '[${o.map(_enc).join(',')}]';
  if (o is String) return '"${o.replaceAll('"', '\\"')}"';
  if (o is bool) return o.toString();
  if (o is num) return o.toString();
  return 'null';
}

class FakeStorage2 extends Fake implements FlutterSecureStorage {
  final Map<String, String> store = {};
  @override
  Future<String?> read({required String key, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store[key];
  @override
  Future<void> write({required String key, required String? value, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store[key] = value ?? '';
  @override
  Future<void> delete({required String key, IOSOptions? iOptions, AndroidOptions? aOptions, LinuxOptions? lOptions, WebOptions? webOptions, MacOsOptions? mOptions, WindowsOptions? wOptions}) async => store.remove(key);
}

ApiClient _api(FakeDioAdapter adapter) {
  final dio = Dio(BaseOptions(baseUrl: 'http://test'));
  dio.httpClientAdapter = adapter;
  return ApiClient(dio: dio, storage: FakeStorage2());
}

void main() {
  group('ApiClient', () {
    test('sends Authorization header when token present', () async {
      final storage = FakeStorage2()..store['access_token'] = 'tok';
      final dio = Dio(BaseOptions(baseUrl: 'http://test'));
      final adapter = FakeDioAdapter(responses: [
        {'body': {'ok': true}},
      ]);
      dio.httpClientAdapter = adapter;
      final api = ApiClient(dio: dio, storage: storage);
      await api.get('/x');
      final options = adapter.requests.first;
      expect(options.headers['Authorization'], 'Bearer tok');
      expect(options.headers['X-Idempotency-Key'], isNotNull);
    });

    test('adds idempotency key even if not provided', () async {
      final dio = Dio(BaseOptions(baseUrl: 'http://test'));
      final adapter = FakeDioAdapter(responses: [
        {'body': {'ok': true}},
      ]);
      dio.httpClientAdapter = adapter;
      final api = ApiClient(dio: dio, storage: FakeStorage2());
      await api.post('/x', data: {'a': 1});
      final options = adapter.requests.first;
      expect(options.headers['X-Idempotency-Key'], isNotNull);
      expect(options.data['a'], 1);
    });

    test('preserves provided idempotency key', () async {
      final dio = Dio(BaseOptions(baseUrl: 'http://test'));
      final adapter = FakeDioAdapter(responses: [
        {'body': {'ok': true}},
      ]);
      dio.httpClientAdapter = adapter;
      final api = ApiClient(dio: dio, storage: FakeStorage2());
      await api.get('/x', headers: {'X-Idempotency-Key': 'my-idem'});
      expect(adapter.requests.first.headers['X-Idempotency-Key'], 'my-idem');
    });
  });

  group('MenuService', () {
    test('fetchMenus and fetchItems', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'menus': [{'menu_id': 'm1', 'name': 'Makanan'}]}}},
        {'body': {'data': {'items': [{'item_id': 'i1', 'name': 'Nasi', 'price': 10000}]}}},
      ]);
      final service = MenuService(_api(adapter));

      final menus = await service.fetchMenus('m1');
      final items = await service.fetchItems('m1');

      expect(menus.single.name, 'Makanan');
      expect(items.single.name, 'Nasi');
    });

    test('fetchMenus handles top-level list', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': [{'id': 'm1', 'name': 'X'}]},
      ]);
      final service = MenuService(_api(adapter));
      final menus = await service.fetchMenus('m1');
      expect(menus.single.id, 'm1');
    });

    test('createMenu and updateMenu and deleteMenu', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'id': 'm1', 'name': 'New'}}},
        {'body': {'data': {'id': 'm1', 'name': 'Updated'}}},
        {'body': {'ok': true}},
      ]);
      final service = MenuService(_api(adapter));

      final created = await service.createMenu('m1', name: 'New', description: 'd', sequenceOrder: 2);
      expect(created.name, 'New');

      final updated = await service.updateMenu('m1', 'm1', name: 'Updated', isActive: false);
      expect(updated.name, 'Updated');

      await service.deleteMenu('m1', 'm1');
      expect(adapter.callCount, 3);
    });

    test('createItem and updateItem and deleteItem', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'id': 'i1', 'name': 'Item', 'price': 1000}}},
        {'body': {'data': {'id': 'i1', 'name': 'Item2', 'price': 2000}}},
        {'body': {'ok': true}},
      ]);
      final service = MenuService(_api(adapter));

      final created = await service.createItem('m1', menuId: 'm1', name: 'Item', price: 1000, stock: 5, isAvailable: false);
      expect(created.name, 'Item');

      final updated = await service.updateItem('m1', 'i1', name: 'Item2', price: 2000, stock: 0, isAvailable: true);
      expect(updated.name, 'Item2');

      await service.deleteItem('m1', 'i1');
      expect(adapter.callCount, 3);
    });
  });

  group('MerchantService', () {
    test('registerMerchant returns data', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'id': 'm1', 'merchant_name': 'Toko'}}},
      ]);
      final service = MerchantService(_api(adapter));
      final d = await service.registerMerchant({'merchant_name': 'Toko'});
      expect(d['id'], 'm1');
    });

    test('registerMerchant throws on invalid response', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': 'notamap'}},
      ]);
      final service = MerchantService(_api(adapter));
      expect(() => service.registerMerchant({}), throwsStateError);
    });

    test('fetchProfile returns profile', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'id': 'm1', 'merchant_name': 'Toko', 'status': 'ACTIVE'}}},
      ]);
      final service = MerchantService(_api(adapter));
      final p = await service.fetchProfile('m1');
      expect(p.merchantName, 'Toko');
      expect(p.isActive, isTrue);
    });

    test('fetchProfile throws on invalid', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': 5}},
      ]);
      final service = MerchantService(_api(adapter));
      expect(() => service.fetchProfile('m1'), throwsStateError);
    });

    test('updateProfile returns updated profile', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'id': 'm1', 'merchant_name': 'New', 'is_open': false, 'status': 'ACTIVE'}}},
      ]);
      final service = MerchantService(_api(adapter));
      final p = await service.updateProfile('m1', isOpen: false, merchantName: 'New');
      expect(p.isOpen, isFalse);
      expect(adapter.requests.first.data['is_open'], false);
    });

    test('updateProfile throws on invalid', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': []}},
      ]);
      final service = MerchantService(_api(adapter));
      expect(() => service.updateProfile('m1'), throwsStateError);
    });
  });

  group('OrderService', () {
    test('fetchOrders parses list', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'orders': [{'id': 'o1', 'status': 'CREATED', 'total_amount': 1000}]}}},
      ]);
      final service = OrderService(_api(adapter));
      final orders = await service.fetchOrders('m1');
      expect(orders.single.id, 'o1');
      expect(orders.single.totalAmount, 1000);
    });

    test('fetchOrders passes status query', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'orders': []}}},
      ]);
      final service = OrderService(_api(adapter));
      await service.fetchOrders('m1', status: 'WAITING');
      expect(adapter.requests.first.path, contains('status=WAITING'));
    });

    test('fetchOrder merges order and items', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'order': {'id': 'o1', 'status': 'PREPARING'}, 'items': [{'item_name': 'Nasi', 'quantity': 1}]}}},
      ]);
      final service = OrderService(_api(adapter));
      final o = await service.fetchOrder('o1');
      expect(o.id, 'o1');
      expect(o.items.single.itemName, 'Nasi');
    });

    test('fetchOrder throws on invalid', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': null}},
      ]);
      final service = OrderService(_api(adapter));
      expect(() => service.fetchOrder('o1'), throwsStateError);
    });

    test('updateStatus returns data or empty map', () async {
      final adapter = FakeDioAdapter(responses: [
        {'body': {'data': {'ok': true}}},
      ]);
      final service = OrderService(_api(adapter));
      final d = await service.updateStatus('o1', 'READY');
      expect(d['ok'], true);

      final adapter2 = FakeDioAdapter(responses: [
        {'body': {'data': 'nope'}},
      ]);
      final service2 = OrderService(_api(adapter2));
      final d2 = await service2.updateStatus('o1', 'READY');
      expect(d2, isEmpty);
    });
  });
}
