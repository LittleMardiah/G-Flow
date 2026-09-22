import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:driver_app/config/constants.dart';
import 'package:driver_app/models/driver_order.dart';
import 'package:driver_app/models/driver_earning.dart';
import 'package:driver_app/services/api_client.dart';
import 'package:driver_app/services/earnings_service.dart';
import 'package:driver_app/services/order_service.dart';

void main() {
  group('OrderService unified endpoint', () {
    test('fetchAvailableOrders parses unified orders + capacity', () async {
      final dio = Dio(BaseOptions(baseUrl: 'http://test'));
      interceptOkJson(dio, {
        'success': true,
        'data': {
          'orders': [
            {
              'id': 'o-ride-1',
              'type': 'ride',
              'status': 'SEARCHING_DRIVER',
              'pickup_address': 'Jl. A',
              'dropoff_label': 'Jl. B',
              'pickup_lat': -6.2,
              'pickup_lng': 106.8,
              'dropoff_lat': -6.25,
              'dropoff_lng': 106.85,
              'distance_km': 2.0,
              'earning': 48000,
              'fare': 60000,
              'payment_method': 'WALLET',
              'created_at': '2026-09-22T10:00:00Z',
            },
            {
              'id': 'o-food-1',
              'type': 'food',
              'status': 'CONFIRMED',
              'pickup_address': 'Warung Sate',
              'dropoff_label': 'Jl. C',
              'distance_km': 1.0,
              'earning': 70000,
              'fare': 70000,
              'payment_method': 'CASH',
              'created_at': '2026-09-22T10:05:00Z',
              'merchant_name': 'Warung Sate',
            },
            {
              'id': 'o-send-1',
              'type': 'send',
              'status': 'SEARCHING_DRIVER',
              'pickup_address': 'Jl. D',
              'dropoff_label': 'Jl. E',
              'distance_km': 3.0,
              'earning': 40500,
              'fare': 45000,
              'payment_method': 'WALLET',
              'created_at': '2026-09-22T10:10:00Z',
            },
          ],
          'capacity_available': true,
          'active_orders': 1,
          'max_active_orders': 3,
          'radius_km': 5,
        },
      });
      final service = OrderService(ApiClient(dio: dio));
      final result = await service.fetchAvailableOrders();

      expect(result.orders, hasLength(3));
      expect(result.capacityAvailable, isTrue);
      expect(result.activeOrders, 1);
      expect(result.maxActiveOrders, kMaxActiveOrders);
      expect(result.radiusKm, 5);
      expect(result.orders.first.type, OrderType.food);
      expect(result.orders.first.deliveryAddress, 'Jl. C');
      expect(result.orders.first.merchantName, 'Warung Sate');
      expect(result.orders.first.estimatedFare, 70000);
      expect(result.orders[1].type, OrderType.ride);
      expect(result.orders[1].estimatedFare, 60000);
      expect(result.orders[1].driverEarning, 48000);
      expect(result.orders.last.type, OrderType.send);
      expect(result.orders.last.deliveryAddress, 'Jl. E');
      expect(result.orders.last.estimatedFare, 45000);
    });

    test('fetchAvailableOrders reports full capacity on capacity_available=false', () async {
      final dio = Dio(BaseOptions(baseUrl: 'http://test'));
      interceptOkJson(dio, {
        'success': true,
        'data': {
          'orders': <Map<String, dynamic>>[],
          'capacity_available': false,
          'active_orders': 3,
          'max_active_orders': 3,
          'radius_km': 5,
        },
      });
      final service = OrderService(ApiClient(dio: dio));
      final result = await service.fetchAvailableOrders();
      expect(result.orders, isEmpty);
      expect(result.capacityAvailable, isFalse);
      expect(result.activeOrders, 3);
      expect(result.radiusKm, 5);
    });

    test('fetchActiveOrders parses drivers/orders response (A2)', () async {
      final dio = Dio(BaseOptions(baseUrl: 'http://test'));
      interceptOkJson(dio, {
        'success': true,
        'data': {
          'orders': [
            {
              'type': 'ride',
              'order_id': 'a-ride-1',
              'status': 'TRIP_STARTED',
              'driver_id': 'd-1',
              'pickup_address': 'Jl. P',
              'pickup_lat': -6.2,
              'pickup_lng': 106.8,
              'dropoff_address': 'Jl. Q',
              'dropoff_lat': -6.25,
              'dropoff_lng': 106.85,
              'distance_km': 2.5,
              'estimated_fare': 75000,
              'payment_method': 'WALLET',
              'created_at': '2026-09-22T10:00:00Z',
            },
            {
              'type': 'food',
              'order_id': 'a-food-1',
              'status': 'PICKED_UP',
              'driver_id': 'd-1',
              'merchant_name': 'Warung',
              'merchant_address': 'Jl. M',
              'merchant_lat': -6.2,
              'merchant_lng': 106.8,
              'delivery_address': 'Jl. R',
              'delivery_lat': -6.25,
              'delivery_lng': 106.85,
              'delivery_fee': 8000,
              'total_amount': 68000,
              'payment_method': 'WALLET',
              'created_at': '2026-09-22T10:10:00Z',
            },
          ],
        },
      });
      final service = OrderService(ApiClient(dio: dio));
      final orders = await service.fetchActiveOrders();

      expect(orders, hasLength(2));
      expect(orders.first.type, OrderType.ride);
      expect(orders.first.id, 'a-ride-1');
      expect(orders.first.status, 'TRIP_STARTED');
      expect(orders.first.estimatedFare, 75000);
      expect(orders.last.type, OrderType.food);
      expect(orders.last.id, 'a-food-1');
      expect(orders.last.merchantName, 'Warung');
      expect(orders.last.deliveryAddress, 'Jl. R');
    });

    test('acceptFood returns true after delay', () async {
      final service = OrderService(ApiClient(dio: Dio(BaseOptions())));
      expect(await service.acceptFood('food-1'), isTrue);
    });
  });

  group('OrderService.fetchRideDetail', () {
    test('falls back to mock order on DioException', () async {
      final dio = Dio(BaseOptions(baseUrl: 'http://unreachable.invalid'));
      intercept404(dio);
      final service = OrderService(ApiClient(dio: dio));
      final order = await service.fetchRideDetail('mock-ride-2351');
      expect(order.id, 'mock-ride-2351');
    });

    test('returns fallback order for unknown id', () async {
      final dio = Dio(BaseOptions(baseUrl: 'http://unreachable.invalid'));
      intercept404(dio);
      final service = OrderService(ApiClient(dio: dio));
      final order = await service.fetchRideDetail('unknown-id');
      expect(order.id, 'unknown-id');
      expect(order.status, 'DRIVER_ASSIGNED');
      expect(order.isMock, isTrue);
    });
  });

  group('EarningsService mock path', () {
    test('fetchEarnings returns mock earning', () async {
      final service = EarningsService(ApiClient(dio: Dio(BaseOptions())));
      final e = await service.fetchEarnings('driver-1');
      expect(e, isA<DriverEarning>());
      expect(e.isMock, isTrue);
      expect(e.daily, isNotEmpty);
      expect(e.weekly.length, 7);
      expect(e.todayTotal, 285000);
      expect(e.todayOrderCount, 12);
      expect(e.weekTotal, 1675000);
      expect(e.rideCount, 6);
      expect(e.foodCount, 5);
      expect(e.sendCount, 1);
      expect(e.totalOrderCount, 12);
    });
  });
}

void intercept404(Dio dio) {
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) =>
          handler.reject(DioException(requestOptions: options, type: DioExceptionType.connectionError)),
    ),
  );
}

void interceptOkJson(Dio dio, Map<String, dynamic> body) {
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) =>
          handler.resolve(Response(requestOptions: options, statusCode: 200, data: body)),
    ),
  );
}
