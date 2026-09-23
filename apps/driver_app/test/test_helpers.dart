import 'package:driver_app/config/constants.dart';
import 'package:driver_app/models/driver_order.dart';
import 'package:driver_app/services/api_client.dart';
import 'package:driver_app/services/order_service.dart';
import 'package:flutter_test/flutter_test.dart';

/// Drains any pending Flutter test exceptions (e.g. failed network image
/// loads from FlutterMap tile layers) so a widget test that renders a map
/// does not fail on network tile fetch errors.
Future<void> drainMapExceptions(WidgetTester tester) async {
  await tester.pump(const Duration(milliseconds: 50));
  for (var i = 0; i < 200; i++) {
    final e = tester.takeException();
    if (e == null) break;
  }
}

class FakeOrderService extends OrderService {
  FakeOrderService() : super(ApiClient());

  List<DriverOrder> availableOrders = [];
  List<DriverOrder> activeOrders = [];
  bool failAccept = false;

  @override
  Future<AvailableOrdersResult> fetchAvailableOrders() async {
    return AvailableOrdersResult(
      orders: availableOrders,
      capacityAvailable: availableOrders.length < kMaxActiveOrders,
      activeOrders: activeOrders.length,
      maxActiveOrders: kMaxActiveOrders,
      radiusKm: 5,
    );
  }

  @override
  Future<List<DriverOrder>> fetchActiveOrders() async => activeOrders;

  @override
  Future<Map<String, dynamic>> acceptRide(String orderId) async {
    if (failAccept) throw Exception('accept fail');
    return const {};
  }

  @override
  Future<Map<String, dynamic>> acceptSend(String orderId) async {
    if (failAccept) throw Exception('accept fail');
    return const {};
  }

  @override
  Future<Map<String, dynamic>> acceptFood(String orderId) async {
    if (failAccept) throw Exception('accept fail');
    return const {};
  }

  @override
  Future<Map<String, dynamic>> updateRideStatus(String orderId, String status) async => const {};

  @override
  Future<Map<String, dynamic>> updateSendStatus(String orderId, String status) async => const {};

  @override
  Future<Map<String, dynamic>> updateFoodStatus(String orderId, String status) async => const {};

  @override
  Future<Map<String, dynamic>> updateSendStop(
    String orderId,
    String stopId, {
    String status = 'COMPLETED',
    String? deliveryPhotoUrl,
  }) async =>
      const {};
}

DriverOrder makeRide(String id, {String status = 'SEARCHING_DRIVER', double km = 1.0}) =>
    DriverOrder.fromRideJson({
      'order_id': id,
      'status': status,
      'pickup_address': 'Jl. A',
      'pickup_lat': -6.2,
      'pickup_lng': 106.8,
      'dropoff_address': 'Jl. B',
      'dropoff_lat': -6.25,
      'dropoff_lng': 106.85,
      'distance_km': km,
      'estimated_fare': 50000,
      'driver_earning': 40000,
    });

DriverOrder makeFood(String id, {String status = 'READY_FOR_PICKUP', double km = 2.0}) =>
    DriverOrder.fromFoodJson({
      'order_id': id,
      'status': status,
      'merchant_name': 'Warung',
      'merchant_address': 'Jl. M',
      'merchant_lat': -6.2,
      'merchant_lng': 106.8,
      'delivery_address': 'Jl. D',
      'delivery_lat': -6.25,
      'delivery_lng': 106.85,
      'distance_km': km,
      'delivery_fee': 8000,
    });

DriverOrder makeSend(String id, {String status = 'IN_TRANSIT', double km = 3.0}) =>
    DriverOrder.fromSendJson({
      'order_id': id,
      'status': status,
      'pickup_address': 'Jl. P',
      'pickup_lat': -6.2,
      'pickup_lng': 106.8,
      'total_distance_km': km,
      'total_fare': 45000,
      'stops': [
        {'stop_id': 's1', 'stop_number': 1, 'recipient_name': 'Budi', 'address': 'Jl. S1', 'allocated_fare': 15000, 'status': 'PENDING', 'lat': -6.21, 'lng': 106.81},
        {'stop_id': 's2', 'stop_number': 2, 'recipient_name': 'Sari', 'address': 'Jl. S2', 'allocated_fare': 19000, 'status': 'PENDING', 'lat': -6.23, 'lng': 106.83},
      ],
    });
