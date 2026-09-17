import 'dart:convert';
import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:latlong2/latlong.dart';
import 'package:uuid/uuid.dart';

import 'package:customer_app/services/api_client.dart';
import 'package:customer_app/services/food_order_service.dart';
import 'package:customer_app/services/merchant_service.dart';
import 'package:customer_app/services/ride_service.dart';
import 'package:customer_app/services/send_order_service.dart';

/// Fake base class (pengganti mockito) agar test tidak butuh dev dependency
/// mockito. Semua method default melempar UnimplementedError; subclass cukup
/// meng-override yang dibutuhkan.
abstract class Fake {
  @override
  NoSuchMethodError noSuchMethod(Invocation invocation) =>
      throw UnimplementedError('${invocation.memberName} not implemented');
}

class DioAdapterMock extends Fake implements HttpClientAdapter {
  final Object? data;
  int statusCode = 200;
  String lastPath = '';
  String lastMethod = '';
  String lastQuery = '';
  Map<String, dynamic> lastHeaders = {};

  DioAdapterMock({this.data, this.statusCode = 200});

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<List<int>>? requestStream, Future<void>? cancelFuture) async {
    lastPath = options.path;
    lastMethod = options.method;
    lastHeaders = options.headers;
    final uri = Uri.parse(options.path);
    lastQuery = uri.query;
    return ResponseBody.fromString(
      jsonEncode(data),
      statusCode,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
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

/// Membuat ApiClient terisolasi: adapter dio dipasang langsung lewat
/// `dio.httpClientAdapter = adapter` (bukan lewat BaseOptions).
ApiClient makeApi(DioAdapterMock adapter) =>
    ApiClient(dio: Dio()..httpClientAdapter = adapter, storage: FakeStorage());

void main() {
  group('RideService', () {
    test('getRideDetails throws StateError on invalid data', () async {
      final dio = DioAdapterMock(data: const {'data': 'not-a-map'});
      final svc = RideService(makeApi(dio));
      await expectLater(svc.getRideDetails('x'), throwsStateError);
    });

    test('getRideDetails parses order', () async {
      final dio = DioAdapterMock(data: {
        'data': {
          'order_id': 'r1',
          'status': 'COMPLETED',
        },
      });
      final svc = RideService(makeApi(dio));
      final order = await svc.getRideDetails('r1');
      expect(order.id, 'r1');
      expect(order.status, 'COMPLETED');
    });

    test('bookRide returns result', () async {
      final dio = DioAdapterMock(data: {
        'data': {
          'order_id': 'r1',
          'status': 'SEARCHING_DRIVER',
          'estimated_fare': 22000,
          'payment_method': 'WALLET',
        },
      });
      final svc = RideService(makeApi(dio));
      final r = await svc.bookRide(
        pickupLat: 1,
        pickupLng: 2,
        dropoffLat: 3,
        dropoffLng: 4,
        paymentMethod: 'WALLET',
        idempotencyKey: const Uuid().v4(),
      );
      expect(r.orderId, 'r1');
      expect(r.estimatedFare, 22000);
    });

    test('cancelRide calls patch', () async {
      final dio = DioAdapterMock(data: const {});
      final svc = RideService(makeApi(dio));
      await svc.cancelRide('r1');
      expect(dio.lastPath, '/api/v1/rides/r1/status');
    });
  });

  group('RideMockSimulator', () {
    test('statusForTick follows sequence', () {
      expect(RideMockSimulator.statusForTick(0), 'SEARCHING_DRIVER');
      expect(RideMockSimulator.statusForTick(2), 'DRIVER_ASSIGNED');
      expect(RideMockSimulator.statusForTick(6), 'DRIVER_ARRIVED');
      expect(RideMockSimulator.statusForTick(10), 'TRIP_STARTED');
      expect(RideMockSimulator.statusForTick(20), 'COMPLETED');
    });

    test('book returns mock result with distance & fare', () {
      final r = RideMockSimulator.book(
        pickupLat: -6.2,
        pickupLng: 106.8,
        dropoffLat: -6.25,
        dropoffLng: 106.85,
        paymentMethod: 'WALLET',
      );
      expect(r.isMock, isTrue);
      expect(r.status, 'SEARCHING_DRIVER');
      expect(r.distanceKm, greaterThan(0));
      expect(r.estimatedFare, greaterThan(10000));
    });

    test('distanceKm returns 0 for identical points', () {
      expect(RideMockSimulator.distanceKm(const LatLng(1, 1), const LatLng(1, 1)), 0);
    });

    test('driverLocationForTick lerps toward dropoff after arrival', () {
      const pickup = LatLng(-6.2, 106.8);
      const dropoff = LatLng(-6.25, 106.85);
      final loc = RideMockSimulator.driverLocationForTick(pickup: pickup, dropoff: dropoff, tick: 20);
      expect(loc.latitude, closeTo(dropoff.latitude, 0.001));
    });
  });

  group('MerchantService', () {
    test('fetchMerchants with lat/lng', () async {
      final dio = DioAdapterMock(data: {
        'data': [
          {'id': 'm1', 'name': 'A'},
          {'id': 'm2', 'name': 'B'},
        ],
      });
      final svc = MerchantService(makeApi(dio));
      final merchants = await svc.fetchMerchants(lat: -6.2, lng: 106.8);
      expect(merchants.length, 2);
      expect(merchants.first.id, 'm1');
    });

    test('fetchMerchants without lat/lng', () async {
      final dio = DioAdapterMock(data: {
        'data': [
          {'id': 'm1', 'name': 'X'},
        ],
      });
      final svc = MerchantService(makeApi(dio));
      final merchants = await svc.fetchMerchants();
      expect(merchants.length, 1);
    });

    test('fetchMerchants with search builds query', () async {
      final dio = DioAdapterMock(data: {'data': <Map<String, dynamic>>[]});
      final svc = MerchantService(makeApi(dio));
      await svc.fetchMerchants(search: 'kopi');
      expect(dio.lastPath, contains('/api/v1/merchants?'));
      expect(dio.lastPath, contains('search=kopi'));
    });

    test('fetchMenus parses items', () async {
      final dio = DioAdapterMock(data: {
        'data': {
          'menus': [
            {'menu_id': 'mu', 'name': 'Main', 'items': [{'id': 'i1', 'merchant_id': 'm1'}]},
          ],
        },
      });
      final svc = MerchantService(makeApi(dio));
      final menus = await svc.fetchMenus('m1');
      expect(menus.length, 1);
      expect(menus.first.items.first.merchantId, 'm1');
    });

    test('fetchMenus empty returns empty', () async {
      final dio = DioAdapterMock(data: {'data': {'menus': <dynamic>[]}});
      final svc = MerchantService(makeApi(dio));
      final menus = await svc.fetchMenus('m1');
      expect(menus, isEmpty);
    });

    test('fetchItemsAsMenus groups by menu_id', () async {
      final dio = DioAdapterMock(data: {
        'data': {
          'items': [
            {'id': 'i1', 'menu_id': 'a'},
            {'id': 'i2', 'menu_id': 'a'},
            {'id': 'i3', 'menu_id': 'b'},
            {'id': 'i4', 'menu_id': null},
          ],
        },
      });
      final svc = MerchantService(makeApi(dio));
      final menus = await svc.fetchItemsAsMenus('m1');
      expect(menus.length, 3);
      final a = menus.firstWhere((m) => m.id == 'a');
      expect(a.items.length, 2);
    });
  });

  group('FoodOrderService', () {
    test('createOrder parses result', () async {
      final dio = DioAdapterMock(data: {'data': {'order_id': 'f1', 'status': 'CREATED'}});
      final svc = FoodOrderService(makeApi(dio));
      final o = await svc.createOrder(
        merchantId: 'm1',
        items: const [{'item_id': 'a'}],
        deliveryAddress: 'addr',
        deliveryLat: 1,
        deliveryLng: 2,
        paymentMethod: 'WALLET',
      );
      expect(o.id, 'f1');
      expect(o.status, 'CREATED');
    });

    test('getOrder throws StateError on invalid', () async {
      final dio = DioAdapterMock(data: {'data': 'nope'});
      final svc = FoodOrderService(makeApi(dio));
      await expectLater(svc.getOrder('f1'), throwsStateError);
    });

    test('getOrder parses map', () async {
      final dio = DioAdapterMock(data: {'data': {'order_id': 'f1', 'status': 'CONFIRMED'}});
      final svc = FoodOrderService(makeApi(dio));
      final o = await svc.getOrder('f1');
      expect(o.id, 'f1');
      expect(o.status, 'CONFIRMED');
    });

    test('updateStatus calls patch', () async {
      final dio = DioAdapterMock(data: const {});
      final svc = FoodOrderService(makeApi(dio));
      await svc.updateStatus('f1', 'CANCELLED');
      expect(dio.lastPath, '/api/v1/food-orders/f1');
    });

    test('getHistory parses orders map', () async {
      final dio = DioAdapterMock(data: {
        'data': {'orders': <Map<String, dynamic>>[{'order_id': 'f1', 'status': 'DELIVERED'}]},
      });
      final svc = FoodOrderService(makeApi(dio));
      final list = await svc.getHistory();
      expect(list.length, 1);
      expect(list.first.id, 'f1');
    });

    test('getHistory handles list root', () async {
      final dio = DioAdapterMock(data: {
        'data': <Map<String, dynamic>>[{'order_id': 'f1'}],
      });
      final svc = FoodOrderService(makeApi(dio));
      final list = await svc.getHistory();
      expect(list.length, 1);
    });
  });

  group('SendOrderService', () {
    test('createOrder parses', () async {
      final dio = DioAdapterMock(data: {'data': {'order_id': 's1', 'status': 'CREATED'}});
      final svc = SendOrderService(makeApi(dio));
      final o = await svc.createOrder(
        packageType: 'STANDARD',
        stops: const [],
        pickupAddress: 'pickup',
        pickupLat: 1,
        pickupLng: 2,
        paymentMethod: 'WALLET',
      );
      expect(o.id, 's1');
      expect(o.status, 'CREATED');
    });

    test('getOrder throws on invalid', () async {
      final dio = DioAdapterMock(data: {'data': 'bad'});
      final svc = SendOrderService(makeApi(dio));
      await expectLater(svc.getOrder('s1'), throwsStateError);
    });

    test('getOrder parses', () async {
      final dio = DioAdapterMock(data: {'data': {'order_id': 's1', 'status': 'IN_TRANSIT'}});
      final svc = SendOrderService(makeApi(dio));
      final o = await svc.getOrder('s1');
      expect(o.id, 's1');
      expect(o.status, 'IN_TRANSIT');
    });

    test('updateStatus calls patch', () async {
      final dio = DioAdapterMock(data: const {});
      final svc = SendOrderService(makeApi(dio));
      await svc.updateStatus('s1', 'CANCELLED');
      expect(dio.lastPath, '/api/v1/send-orders/s1');
    });

    test('getHistory parses', () async {
      final dio = DioAdapterMock(data: {
        'data': {'orders': <Map<String, dynamic>>[{'order_id': 's1', 'status': 'DELIVERED'}]},
      });
      final svc = SendOrderService(makeApi(dio));
      final list = await svc.getHistory();
      expect(list.length, 1);
    });
  });

  group('ApiClient', () {
    test('post returns response', () async {
      final dio = DioAdapterMock(data: {'ok': true});
      final api = makeApi(dio);
      final res = await api.post('/api/v1/x', data: const {'a': 1});
      expect(res.data['ok'], isTrue);
      expect(dio.lastPath, '/api/v1/x');
    });

    test('get returns response', () async {
      final dio = DioAdapterMock(data: const {'k': 'v'});
      final api = makeApi(dio);
      final res = await api.get('/api/v1/y');
      expect(res.data['k'], 'v');
    });

    test('patch returns response', () async {
      final dio = DioAdapterMock(data: const {});
      final api = makeApi(dio);
      await api.patch('/api/v1/z', data: const {'s': 1});
      expect(dio.lastMethod, 'PATCH');
    });

    test('delete returns response', () async {
      final dio = DioAdapterMock(data: const {});
      final api = makeApi(dio);
      await api.delete('/api/v1/d');
      expect(dio.lastMethod, 'DELETE');
    });

    test('interceptor attaches idempotency key when missing', () async {
      final dio = DioAdapterMock(data: const {});
      final api = makeApi(dio);
      await api.get('/api/v1/ping');
      final key = dio.lastHeaders['X-Idempotency-Key'];
      expect(key, isNotNull);
      expect((key as String).trim(), isNotEmpty);
    });

    test('interceptor attaches bearer token when stored', () async {
      final dio = DioAdapterMock(data: const {});
      final storage = FakeStorage()..store['access_token'] = 'tok';
      final api = ApiClient(dio: Dio()..httpClientAdapter = dio, storage: storage);
      await api.get('/api/v1/me');
      expect(dio.lastHeaders['Authorization'], 'Bearer tok');
    });

    test('interceptor deletes token on 401', () async {
      final dio = DioAdapterMock(data: const {}, statusCode: 401);
      final storage = FakeStorage()..store['access_token'] = 'tok';
      final api = ApiClient(dio: Dio()..httpClientAdapter = dio, storage: storage);
      await expectLater(api.get('/api/v1/me'), throwsA(isA<DioException>()));
      expect(storage.store.containsKey('access_token'), isFalse);
    });
  });
}
