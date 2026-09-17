import 'package:flutter_test/flutter_test.dart';
import 'package:latlong2/latlong.dart';

import 'package:customer_app/models/food_order.dart';
import 'package:customer_app/models/food_order_item.dart';
import 'package:customer_app/models/item_option.dart';
import 'package:customer_app/models/menu.dart';
import 'package:customer_app/models/menu_item.dart';
import 'package:customer_app/models/merchant.dart';
import 'package:customer_app/models/option_choice.dart';
import 'package:customer_app/models/ride_order.dart';
import 'package:customer_app/models/send_order.dart';
import 'package:customer_app/models/send_stop.dart';

void main() {
  group('SendStop', () {
    test('fromJson full fields', () {
      final s = SendStop.fromJson(const {
        'stop_id': 's1',
        'order_id': 'o1',
        'order': 2,
        'recipient_name': 'Budi',
        'address': 'Jl A',
        'location_lat': -6.2,
        'location_lng': 106.8,
        'distance_km': 3.5,
        'allocated_fare': 12000,
        'status': 'COMPLETED',
      });
      expect(s.id, 's1');
      expect(s.orderId, 'o1');
      expect(s.order, 2);
      expect(s.recipientName, 'Budi');
      expect(s.latitude, -6.2);
      expect(s.longitude, 106.8);
      expect(s.distanceKm, 3.5);
      expect(s.allocatedFare, 12000);
      expect(s.isDelivered, isTrue);
      expect(s.isSkipped, isFalse);
    });

    test('fromJson alternate aliases & default status', () {
      final s = SendStop.fromJson(const {
        'id': 'sid',
        'name': 'Ani',
        'lat': -6.1,
        'lng': 106.7,
        'stop_order': 3,
        'status': 'SKIPPED',
      });
      expect(s.id, 'sid');
      expect(s.status, 'SKIPPED');
      expect(s.isSkipped, isTrue);
      expect(s.isDelivered, isFalse);
      expect(s.order, 3);
    });

    test('fromJson null-ish values default', () {
      final s = SendStop.fromJson(const {});
      expect(s.id, '');
      expect(s.status, 'PENDING');
      expect(s.latitude, 0);
      expect(s.longitude, 0);
      expect(s.distanceKm, 0);
      expect(s.allocatedFare, 0);
      expect(s.isDelivered, isFalse);
    });
  });

  group('SendOrder', () {
    const full = {
      'order_id': 'so1',
      'customer_id': 'c1',
      'driver_id': 'd1',
      'status': 'IN_TRANSIT',
      'package_type': 'FRAGILE',
      'weight_kg': 7.5,
      'total_distance_km': 12.0,
      'estimated_fare': 40000,
      'total_amount': 40000,
      'payment_method': 'CASH',
      'pickup_address': 'Jl P',
      'created_at': '2025-01-01T10:00:00Z',
      'stops': [
        {'stop_id': 'x', 'status': 'COMPLETED'},
      ],
    };

    test('fromJson full + isMock flag', () {
      final o = SendOrder.fromJson(full, isMock: true);
      expect(o.id, 'so1');
      expect(o.customerId, 'c1');
      expect(o.driverId, 'd1');
      expect(o.status, 'IN_TRANSIT');
      expect(o.packageType, 'FRAGILE');
      expect(o.weightKg, 7.5);
      expect(o.totalDistanceKm, 12.0);
      expect(o.estimatedFare, 40000);
      expect(o.totalAmount, 40000);
      expect(o.paymentMethod, 'CASH');
      expect(o.pickupAddress, 'Jl P');
      expect(o.createdAt, isNotNull);
      expect(o.stops.length, 1);
      expect(o.isMock, isTrue);
      expect(o.stopsCompleted, 1);
      expect(o.isTerminal, isFalse);
    });

    test('sender_id alias + default status', () {
      final o = SendOrder.fromJson(const {'id': 'a', 'sender_id': 's', 'status': 'DELIVERED'});
      expect(o.customerId, 's');
      expect(o.status, 'DELIVERED');
      expect(o.isTerminal, isTrue);
    });

    test('terminal + currentStepIndex', () {
      final o = SendOrder.fromJson(const {'id': 'b', 'status': 'SETTLED'});
      expect(o.isTerminal, isTrue);
      expect(o.currentStepIndex, kSendStatusFlow.indexOf('SETTLED'));
      final c = SendOrder.fromJson(const {'id': 'c'});
      expect(c.status, 'SEARCHING_DRIVER');
      expect(c.currentStepIndex, 1);
    });

    test('fromJson ignores non-map stops', () {
      final o = SendOrder.fromJson(const {'id': 'd', 'stops': ['bad', 1]});
      expect(o.stops, isEmpty);
    });

    test('SendOrderTrackingArgs defaults', () {
      const args = SendOrderTrackingArgs(orderId: 'x');
      expect(args.totalAmount, 0);
      expect(args.packageType, 'STANDARD');
      expect(args.paymentMethod, 'WALLET');
      expect(args.isMock, isFalse);
    });
  });

  group('RideOrder', () {
    const full = {
      'order_id': 'r1',
      'driver_id': 'd1',
      'status': 'DRIVER_ARRIVED',
      'pickup_lat': -6.2,
      'pickup_lng': 106.8,
      'dropoff_lat': -6.3,
      'dropoff_lng': 106.9,
      'pickup_address': 'Jl P',
      'dropoff_address': 'Jl D',
      'distance_km': 5.0,
      'estimated_fare': 30000,
      'payment_method': 'CASH',
      'created_at': '2025-01-01T10:00:00Z',
      'cancellation_reason': 'CUSTOMER_CANCEL',
    };

    test('fromJson full', () {
      final o = RideOrder.fromJson(full);
      expect(o.id, 'r1');
      expect(o.driverId, 'd1');
      expect(o.status, 'DRIVER_ARRIVED');
      expect(o.pickupLat, -6.2);
      expect(o.dropoffLng, 106.9);
      expect(o.pickupAddress, 'Jl P');
      expect(o.distanceKm, 5.0);
      expect(o.estimatedFare, 30000);
      expect(o.createdAt, isNotNull);
      expect(o.cancellationReason, 'CUSTOMER_CANCEL');
    });

    test('pickup/dropoff LatLng getters', () {
      final o = RideOrder.fromJson(full);
      expect(o.pickup, LatLng(-6.2, 106.8));
      expect(o.dropoff, LatLng(-6.3, 106.9));
    });

    test('isTerminal & currentStepIndex', () {
      final o = RideOrder(id: 'x', status: 'COMPLETED', pickupLat: 0, pickupLng: 0, dropoffLat: 1, dropoffLng: 1);
      expect(o.isTerminal, isTrue);
      expect(o.currentStepIndex, 4);
      final c = RideOrder(id: 'y', status: 'SEARCHING_DRIVER', pickupLat: 0, pickupLng: 0, dropoffLat: 1, dropoffLng: 1);
      expect(c.isTerminal, isFalse);
      expect(c.currentStepIndex, 0);
      final s = RideOrder(id: 'z', status: 'SETTLED', pickupLat: 0, pickupLng: 0, dropoffLat: 1, dropoffLng: 1);
      expect(s.isTerminal, isTrue);
    });

    test('copyWith', () {
      final o = RideOrder(id: 'x', status: 'SEARCHING_DRIVER', pickupLat: 0, pickupLng: 0, dropoffLat: 1, dropoffLng: 1);
      final c = o.copyWith(status: 'COMPLETED', cancellationReason: 'r');
      expect(c.status, 'COMPLETED');
      expect(c.cancellationReason, 'r');
      expect(c.id, 'x');
      expect(c.pickupLat, 0);
      final d = o.copyWith();
      expect(d.status, 'SEARCHING_DRIVER');
      expect(d.cancellationReason, isNull);
    });

    test('fromJson default status', () {
      final o = RideOrder.fromJson(const {'id': 'a'});
      expect(o.status, 'SEARCHING_DRIVER');
      expect(o.pickupLat, 0);
      expect(o.paymentMethod, 'WALLET');
    });
  });

  group('DriverInfo', () {
    test('fromJson full', () {
      final d = DriverInfo.fromJson(const {
        'driver_id': 'd1',
        'name': 'Ahmad',
        'phone': '0812',
        'vehicle_type': 'Avanza',
        'vehicle_plate': 'B 1',
        'photo_url': 'http://x',
        'rating': 4.8,
        'review_count': 10,
      });
      expect(d.id, 'd1');
      expect(d.name, 'Ahmad');
      expect(d.phone, '0812');
      expect(d.vehicleType, 'Avanza');
      expect(d.vehiclePlate, 'B 1');
      expect(d.photoUrl, 'http://x');
      expect(d.rating, 4.8);
      expect(d.reviewCount, 10);
    });

    test('fromJson aliases & defaults', () {
      final d = DriverInfo.fromJson(const {'id': 'd', 'full_name': 'B'});
      expect(d.id, 'd');
      expect(d.name, 'B');
      expect(d.rating, 0);
      expect(d.reviewCount, 0);
    });
  });

  group('BookRideResult', () {
    test('fromJson + isMock flag', () {
      final r = BookRideResult.fromJson(const {
        'order_id': 'o1',
        'status': 'SEARCHING_DRIVER',
        'distance_km': 4.5,
        'estimated_fare': 28000,
        'payment_method': 'WALLET',
      }, isMock: true);
      expect(r.orderId, 'o1');
      expect(r.status, 'SEARCHING_DRIVER');
      expect(r.distanceKm, 4.5);
      expect(r.estimatedFare, 28000);
      expect(r.paymentMethod, 'WALLET');
      expect(r.isMock, isTrue);
    });

    test('fromJson defaults', () {
      final r = BookRideResult.fromJson(const {});
      expect(r.orderId, '');
      expect(r.status, 'SEARCHING_DRIVER');
      expect(r.distanceKm, 0);
      expect(r.estimatedFare, 0);
      expect(r.paymentMethod, 'WALLET');
      expect(r.isMock, isFalse);
    });
  });

  group('OptionChoice', () {
    test('fromJson with id & price_adjustment', () {
      final c = OptionChoice.fromJson(const {'option_id': 'o', 'name': 'N', 'price_adjustment': 1000});
      expect(c.id, 'o');
      expect(c.name, 'N');
      expect(c.priceAdjustment, 1000);
    });
    test('fromJson alias price_adjust', () {
      final c = OptionChoice.fromJson(const {'id': 'o', 'price_adjust': 500});
      expect(c.priceAdjustment, 500);
    });
    test('fromJson defaults', () {
      final c = OptionChoice.fromJson(const {});
      expect(c.id, '');
      expect(c.name, '');
      expect(c.priceAdjustment, 0);
    });
  });

  group('Merchant', () {
    test('fromJson alternate fields & closed', () {
      final m = Merchant.fromJson(const {
        'id': 'm1',
        'name': 'X',
        'category': 'Kopi',
        'lat': -6.1,
        'lng': 106.7,
        'address': 'Jl',
        'rating': 3.2,
        'is_online': false,
        'profile_photo': 'http://p',
        'delivery_fee': 5000,
        'average_delivery_time': 30,
        'distance_km': 1.2,
        'review_count': 7,
      });
      expect(m.id, 'm1');
      expect(m.name, 'X');
      expect(m.latitude, -6.1);
      expect(m.isOpen, isFalse);
      expect(m.logoUrl, 'http://p');
      expect(m.deliveryFee, 5000);
      expect(m.reviewCount, 7);
    });

    test('fromJson is_open false disables', () {
      final m = Merchant.fromJson(const {'id': 'm', 'is_open': false});
      expect(m.isOpen, isFalse);
      final m2 = Merchant.fromJson(const {'id': 'm', 'is_online': false});
      expect(m2.isOpen, isFalse);
    });

    test('fromJson empty defaults open', () {
      final m = Merchant.fromJson(const {'id': 'm'});
      expect(m.isOpen, isTrue);
      expect(m.name, '');
      expect(m.longitude, 0);
      expect(m.rating, 0);
    });
  });

  group('MenuItem', () {
    test('is_available false', () {
      final i = MenuItem.fromJson(const {'id': 'i', 'is_available': false});
      expect(i.isAvailable, isFalse);
      final j = MenuItem.fromJson(const {'id': 'i', 'available': false});
      expect(j.isAvailable, isFalse);
    });

    test('fromJson alias + options parsed', () {
      final i = MenuItem.fromJson({
        'id': 'i',
        'merchant_id': 'm',
        'menu_id': 'mu',
        'name': 'N',
        'price': 100,
        'option_groups': [
          {'id': 'g', 'name': 'G', 'options': [{'id': 'o'}]},
        ],
      });
      expect(i.merchantId, 'm');
      expect(i.menuId, 'mu');
      expect(i.name, 'N');
      expect(i.options.length, 1);
    });

    test('fromJson empty', () {
      final i = MenuItem.fromJson(const {});
      expect(i.id, '');
      expect(i.name, '');
      expect(i.price, 0);
      expect(i.options, isEmpty);
      expect(i.isAvailable, isTrue);
    });
  });

  group('Menu', () {
    test('fromJson nested items with merchantId', () {
      final m = Menu.fromJson(const {
        'menu_id': 'mu',
        'merchant_id': 'mer',
        'name': 'Menu',
        'items': [
          {'id': 'i1', 'name': 'A'},
          {'id': 'i2', 'name': 'B'},
        ],
      }, merchantId: 'mer2');
      expect(m.id, 'mu');
      expect(m.name, 'Menu');
      expect(m.merchantId, 'mer2');
      expect(m.items.length, 2);
      expect(m.items.first.merchantId, 'mer2');
    });

    test('fromJson empty', () {
      final m = Menu.fromJson(const {});
      expect(m.id, '');
      expect(m.items, isEmpty);
    });

    test('fromJson ignores non-map items', () {
      final m = Menu.fromJson(const {'id': 'm', 'items': ['x', 5]});
      expect(m.items, isEmpty);
    });
  });

  group('ItemOptionGroup', () {
    test('isMultiple true', () {
      final g = ItemOptionGroup.fromJson(const {
        'id': 'g',
        'selection_type': 'MULTIPLE',
        'is_required': true,
        'options': [{'id': 'o'}],
      });
      expect(g.isMultiple, isTrue);
      expect(g.isRequired, isTrue);
      expect(g.options.length, 1);
    });

    test('selectionType camel alias & required false', () {
      final g = ItemOptionGroup.fromJson(const {'id': 'g', 'selectionType': 'MULTIPLE', 'required': false});
      expect(g.isMultiple, isTrue);
      expect(g.isRequired, isFalse);
    });

    test('defaults single & not required', () {
      final g = ItemOptionGroup.fromJson(const {'id': 'g'});
      expect(g.isMultiple, isFalse);
      expect(g.isRequired, isFalse);
      expect(g.options, isEmpty);
    });
  });

  group('FoodOrderItem', () {
    test('fromJson string options', () {
      final i = FoodOrderItem.fromJson(const {
        'order_item_id': 'oi',
        'item_name': 'N',
        'quantity': 2,
        'unit_price': 100,
        'subtotal': 200,
        'selected_options': 'A, B , C',
      });
      expect(i.orderItemId, 'oi');
      expect(i.itemName, 'N');
      expect(i.quantity, 2);
      expect(i.price, 100);
      expect(i.subtotal, 200);
      expect(i.selectedOptions, ['A', 'B', 'C']);
    });

    test('fromJson list options & price alias', () {
      final i = FoodOrderItem.fromJson(const {
        'name': 'N',
        'price': 50,
        'selected_options': ['X', 'Y'],
      });
      expect(i.itemName, 'N');
      expect(i.price, 50);
      expect(i.selectedOptions, ['X', 'Y']);
    });

    test('fromJson empty', () {
      final i = FoodOrderItem.fromJson(const {});
      expect(i.itemName, '');
      expect(i.quantity, 0);
      expect(i.selectedOptions, isEmpty);
      expect(i.subtotal, 0);
    });
  });

  group('FoodOrder', () {
    test('currentStepIndex & isTerminal', () {
      final o = FoodOrder(id: 'x', status: 'PREPARING');
      expect(o.currentStepIndex, 2);
      expect(o.isTerminal, isFalse);
      final d = FoodOrder(id: 'y', status: 'DELIVERED');
      expect(d.isTerminal, isTrue);
      final s = FoodOrder(id: 'z', status: 'SETTLED');
      expect(s.isTerminal, isTrue);
      final c = FoodOrder(id: 'w', status: 'CANCELLED');
      expect(c.isTerminal, isTrue);
    });

    test('fromJson with dates & isMock', () {
      final o = FoodOrder.fromJson(const {
        'order_id': 'f1',
        'merchant_id': 'm1',
        'customer_id': 'c1',
        'status': 'READY_FOR_PICKUP',
        'subtotal_amount': 100,
        'delivery_fee': 10,
        'discount_amount': 5,
        'total_amount': 105,
        'payment_method': 'CASH',
        'delivery_address': 'Jl',
        'created_at': '2025-01-01T10:00:00Z',
        'status_updated_at': '2025-01-01T11:00:00Z',
        'estimated_delivery': '2025-01-01T12:00:00Z',
      }, isMock: true);
      expect(o.id, 'f1');
      expect(o.merchantId, 'm1');
      expect(o.customerId, 'c1');
      expect(o.status, 'READY_FOR_PICKUP');
      expect(o.subtotalAmount, 100);
      expect(o.deliveryFee, 10);
      expect(o.discountAmount, 5);
      expect(o.totalAmount, 105);
      expect(o.paymentMethod, 'CASH');
      expect(o.deliveryAddress, 'Jl');
      expect(o.createdAt, isNotNull);
      expect(o.statusUpdatedAt, isNotNull);
      expect(o.estimatedDelivery, isNotNull);
      expect(o.isMock, isTrue);
      expect(o.currentStepIndex, 3);
    });

    test('fromJson default status CREATED', () {
      final o = FoodOrder.fromJson(const {'id': 'a'});
      expect(o.status, 'CREATED');
      expect(o.currentStepIndex, 0);
    });

    test('fromJson ignores non-map items', () {
      final o = FoodOrder.fromJson(const {'id': 'b', 'items': ['x']});
      expect(o.items, isEmpty);
    });

    test('FoodOrderTrackingArgs defaults', () {
      const args = FoodOrderTrackingArgs(orderId: 'x');
      expect(args.merchantId, isNull);
      expect(args.merchantName, '');
      expect(args.totalAmount, 0);
      expect(args.isMock, isFalse);
    });
  });
}
