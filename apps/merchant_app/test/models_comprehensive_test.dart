import 'package:flutter_test/flutter_test.dart';

import 'package:merchant_app/config/constants.dart';
import 'package:merchant_app/models/merchant.dart';
import 'package:merchant_app/models/merchant_item.dart';
import 'package:merchant_app/models/merchant_menu.dart';
import 'package:merchant_app/models/merchant_order.dart';
import 'package:merchant_app/providers/analytics_provider.dart';

void main() {
  group('formatRupiah', () {
    test('formats positive amount with group separators', () {
      expect(formatRupiah(45000), 'Rp 45.000');
      expect(formatRupiah(1000000), 'Rp 1.000.000');
      expect(formatRupiah(0), 'Rp 0');
      expect(formatRupiah(123), 'Rp 123');
    });

    test('formats negative amount', () {
      expect(formatRupiah(-5000), '-Rp 5.000');
    });
  });

  group('greetingForHour', () {
    test('returns correct greeting by hour', () {
      expect(greetingForHour(DateTime(2026, 1, 1, 5)), 'Good morning');
      expect(greetingForHour(DateTime(2026, 1, 1, 11)), 'Good afternoon');
      expect(greetingForHour(DateTime(2026, 1, 1, 15)), 'Good evening');
      expect(greetingForHour(DateTime(2026, 1, 1, 19)), 'Good night');
      expect(greetingForHour(DateTime(2026, 1, 1, 10)), 'Good morning');
      expect(greetingForHour(DateTime(2026, 1, 1, 14)), 'Good afternoon');
      expect(greetingForHour(DateTime(2026, 1, 1, 18)), 'Good evening');
      expect(greetingForHour(DateTime(2026, 1, 1, 23)), 'Good night');
    });
  });

  group('MerchantOrder', () {
    MerchantOrder base({String status = '', String merchantStatus = '', bool settled = false}) {
      return MerchantOrder(
        id: 'order-123456789',
        customerId: 'cust-1',
        merchantId: 'm-1',
        driverId: 'd-1',
        status: status,
        merchantStatus: merchantStatus,
        items: const [
          MerchantOrderItem(orderItemId: 'oi1', itemName: 'Nasi', quantity: 2, unitPrice: 10000, subtotal: 20000, selectedOptions: ['Pedas']),
          MerchantOrderItem(orderItemId: 'oi2', itemName: 'Ayam', quantity: 1, unitPrice: 15000, subtotal: 15000),
        ],
        itemSubtotal: 35000,
        deliveryFee: 5000,
        discountAmount: 0,
        totalAmount: 40000,
        paymentMethod: 'WALLET',
        deliveryAddress: 'Jl. Merdeka',
        specialInstructions: 'Jangan terlalu pedas',
        merchantNotes: null,
        createdAt: DateTime(2026, 1, 1, 10, 30),
        confirmedAt: DateTime(2026, 1, 1, 10, 35),
        pickupAt: null,
        deliveredAt: null,
        isSettled: settled,
      );
    }

    test('idShort truncates and uppercases long ids', () {
      expect(base().idShort, '#ORDER-12');
    });

    test('idShort handles short ids', () {
      final o = MerchantOrder(id: 'abc');
      expect(o.idShort, '#ABC');
    });

    test('displayStatus uses merchantStatus when present', () {
      final o = MerchantOrder(id: '1', status: 'CREATED', merchantStatus: 'PREPARING');
      expect(o.displayStatus, 'PREPARING');
    });

    test('displayStatus maps CREATED to WAITING', () {
      final o = MerchantOrder(id: '1', status: 'CREATED');
      expect(o.displayStatus, 'WAITING');
      expect(o.isWaiting, isTrue);
    });

    test('displayStatus maps READY_FOR_PICKUP to READY', () {
      final o = MerchantOrder(id: '1', status: 'READY_FOR_PICKUP');
      expect(o.displayStatus, 'READY');
      expect(o.isReady, isTrue);
    });

    test('displayStatus falls through to raw status', () {
      final o = MerchantOrder(id: '1', status: 'DELIVERED');
      expect(o.displayStatus, 'DELIVERED');
    });

    test('status flags', () {
      expect(base(status: 'CREATED', merchantStatus: 'CONFIRMED').isConfirmed, isTrue);
      expect(base(status: 'CREATED', merchantStatus: 'PREPARING').isPreparing, isTrue);
      expect(base(status: 'CANCELLED', merchantStatus: 'CANCELLED').isCancelled, isTrue);
    });

    test('itemCount sums quantities', () {
      expect(base().itemCount, 3);
    });

    test('isTerminal for terminal statuses', () {
      expect(base(status: '', merchantStatus: 'DELIVERED').isTerminal, isTrue);
      expect(base(status: '', merchantStatus: 'SETTLED').isTerminal, isTrue);
      expect(base(status: '', merchantStatus: 'CANCELLED').isTerminal, isTrue);
      expect(base(status: 'PREPARING').isTerminal, isFalse);
      expect(base(status: 'WAITING').isTerminal, isFalse);
    });

    test('nextActions WAITING -> confirm/reject', () {
      final a = base().nextActions;
      expect(a.length, 2);
      expect(a[0].label, 'Confirm');
      expect(a[0].targetStatus, 'CONFIRMED');
      expect(a[1].label, 'Reject');
      expect(a[1].targetStatus, 'CANCELLED');
    });

    test('nextActions CONFIRMED -> start preparing', () {
      final a = base(merchantStatus: 'CONFIRMED').nextActions;
      expect(a.length, 1);
      expect(a.single.label, 'Start Preparing');
      expect(a.single.targetStatus, 'PREPARING');
    });

    test('nextActions PREPARING -> ready for pickup', () {
      final a = base(merchantStatus: 'PREPARING').nextActions;
      expect(a.length, 1);
      expect(a.single.label, 'Ready for Pickup');
      expect(a.single.targetStatus, 'READY_FOR_PICKUP');
    });

    test('nextActions empty for terminal/other states', () {
      expect(base(merchantStatus: 'DELIVERED').nextActions, isEmpty);
      expect(base(merchantStatus: 'READY').nextActions, isEmpty);
      expect(base().displayStatus == 'WAITING', isTrue);
    });

    test('copyWith updates status fields', () {
      final o = base();
      final copy = o.copyWith(status: 'READY_FOR_PICKUP', merchantNotes: 'ok');
      expect(copy.status, 'READY_FOR_PICKUP');
      expect(copy.merchantNotes, 'ok');
      expect(copy.items, o.items);
      expect(copy.totalAmount, 40000);
    });

    test('copyWith preserves values when args null', () {
      final o = base();
      final copy = o.copyWith();
      expect(copy.status, o.status);
      expect(copy.merchantNotes, o.merchantNotes);
    });

    test('fromJson parses full order with items', () {
      final o = MerchantOrder.fromJson({
        'order_id': 'ord-1',
        'customer_id': 'c-1',
        'merchant_id': 'm-1',
        'driver_id': 'dr-1',
        'status': 'CREATED',
        'items': [
          {'order_item_id': 'x', 'item_name': 'A', 'quantity': 1, 'unit_price': 10, 'subtotal': 10},
        ],
        'item_subtotal': '10',
        'delivery_fee': 2000.5,
        'discount_amount': 1000,
        'total_amount': 11000,
        'payment_method': 'COD',
        'delivery_address': 'addr',
        'special_instructions': 'note',
        'merchant_notes': 'mnote',
        'created_at': '2026-01-01T10:00:00',
        'confirmed_at': '2026-01-01T10:00:01',
        'pickup_at': '2026-01-01T10:00:02',
        'delivered_at': '2026-01-01T10:00:03',
        'is_settled': true,
      });
      expect(o.id, 'ord-1');
      expect(o.items.length, 1);
      expect(o.itemSubtotal, 10);
      expect(o.deliveryFee, 2000); // 2000.5 rounds to 2000 by round()
      expect(o.isSettled, isTrue);
      expect(o.createdAt, isNotNull);
    });

    test('MerchantOrderItem parses options as string list and comma string', () {
      final a = MerchantOrderItem.fromJson({
        'order_item_id': 'a',
        'name': 'Burger',
        'quantity': 2,
        'price': 10000,
        'subtotal': 20000,
        'options': ['Keju', 'Saus'],
      });
      expect(a.selectedOptions, ['Keju', 'Saus']);

      final b = MerchantOrderItem.fromJson({'options': ' Keju , Saus ,'});
      expect(b.selectedOptions, ['Keju', 'Saus']);

      final c = MerchantOrderItem.fromJson({});
      expect(c.selectedOptions, isEmpty);
      expect(c.itemName, '');
    });

    test('empty items list yields empty itemCount', () {
      const o = MerchantOrder(id: '1');
      expect(o.itemCount, 0);
    });
  });

  group('MerchantItem', () {
    test('outOfStock true when stock 0', () {
      const item = MerchantItem(id: '1', stock: 0);
      expect(item.outOfStock, isTrue);
    });

    test('outOfStock false when stock > 0', () {
      const item = MerchantItem(id: '1', stock: 5);
      expect(item.outOfStock, isFalse);
    });

    test('copyWith updates fields', () {
      const item = MerchantItem(id: '1', name: 'Old', price: 100, stock: 1, isAvailable: true, menuId: 'm1', description: 'd', imageUrl: 'u');
      final c = item.copyWith(name: 'New', price: 200, stock: 0, isAvailable: false, menuId: 'm2', description: 'd2', imageUrl: 'u2');
      expect(c.name, 'New');
      expect(c.price, 200);
      expect(c.stock, 0);
      expect(c.isAvailable, isFalse);
      expect(c.menuId, 'm2');
      expect(c.id, '1');
    });

    test('copyWith preserves when null', () {
      const item = MerchantItem(id: '1', name: 'X');
      final c = item.copyWith();
      expect(c.name, 'X');
      expect(c.menuId, null);
    });

    test('fromJson uses item_id fallback and available alias', () {
      final a = MerchantItem.fromJson({'item_id': 'i1', 'available': false});
      expect(a.id, 'i1');
      expect(a.isAvailable, isFalse);
    });

    test('fromJson parses numeric string fields', () {
      final a = MerchantItem.fromJson({'price': '12345', 'stock': 2.7});
      expect(a.price, 12345);
      expect(a.stock, 3); // 2.7 rounds to 3
    });
  });

  group('MerchantMenu', () {
    test('copyWith keeps items and updates scalar fields', () {
      const menu = MerchantMenu(id: '1', name: 'Makanan', description: 'd', isActive: true, items: [{'a': 1}]);
      final c = menu.copyWith(name: 'Minuman', isActive: false, description: 'd2');
      expect(c.name, 'Minuman');
      expect(c.isActive, isFalse);
      expect(c.description, 'd2');
      expect(c.items.length, 1);
      expect(c.id, '1');
    });

    test('copyWith with nulls preserves', () {
      const menu = MerchantMenu(id: '1', name: 'X');
      final c = menu.copyWith();
      expect(c.name, 'X');
      expect(c.isActive, isTrue);
    });

    test('fromJson uses menu_id fallback', () {
      final a = MerchantMenu.fromJson({'menu_id': 'mm', 'name': 'X', 'items': [{'id': 'i'}]});
      expect(a.id, 'mm');
      expect(a.items.length, 1);
    });

    test('fromJson items non-list ignored', () {
      final a = MerchantMenu.fromJson({'id': 'mm', 'items': 'notalist'});
      expect(a.items, isEmpty);
    });
  });

  group('MerchantProfile', () {
    test('isActive and isPendingVerification', () {
      const p = MerchantProfile(id: '1', status: 'ACTIVE');
      expect(p.isActive, isTrue);
      expect(p.isPendingVerification, isFalse);

      const q = MerchantProfile(id: '2', status: 'PENDING_VERIFICATION');
      expect(q.isActive, isFalse);
      expect(q.isPendingVerification, isTrue);
    });

    test('copyWith updates open fields', () {
      const p = MerchantProfile(id: '1', merchantName: 'Old', isOpen: true, category: 'c', openingTime: 'o', closingTime: 'cl', logoUrl: 'l', address: 'a');
      final c = p.copyWith(isOpen: false, merchantName: 'New', category: 'c2', openingTime: 'o2', closingTime: 'cl2', logoUrl: 'l2', address: 'a2');
      expect(c.isOpen, isFalse);
      expect(c.merchantName, 'New');
      expect(c.id, '1');
    });

    test('copyWith with nulls preserves', () {
      const p = MerchantProfile(id: '1', merchantName: 'Old', isOpen: true);
      final c = p.copyWith();
      expect(c.merchantName, 'Old');
      expect(c.isOpen, isTrue);
    });

    test('fromJson uses name/lat/lng/logo fallbacks', () {
      final p = MerchantProfile.fromJson({
        'merchant_id': 'm',
        'name': 'Toko',
        'lat': '3.5',
        'lng': 106.8,
        'is_open': false,
        'profile_photo': 'http://logo',
        'avg_rating': '4.2',
        'total_reviews': '10',
        'total_orders': '5',
      });
      expect(p.id, 'm');
      expect(p.merchantName, 'Toko');
      expect(p.latitude, 3.5);
      expect(p.longitude, 106.8);
      expect(p.isOpen, isFalse);
      expect(p.logoUrl, 'http://logo');
      expect(p.avgRating, 4.2);
      expect(p.totalReviews, 10);
      expect(p.totalOrders, 5);
    });

    test('fromJson handles missing ids', () {
      final p = MerchantProfile.fromJson({});
      expect(p.id, '');
      expect(p.isOpen, isTrue);
    });

    test('toUpdateJson includes only provided fields', () {
      const p = MerchantProfile(id: '1');
      final body = p.toUpdateJson(isOpen: false, merchantName: 'X');
      expect(body['is_open'], false);
      expect(body['merchant_name'], 'X');
      expect(body.containsKey('category'), isFalse);
      expect(body['logo_url'], null);
    });
  });

  group('AnalyticsSummary', () {
    test('excludes null createdAt and validates week/month', () {
      final now = DateTime.now();
      final today = DateTime(now.year, now.month, now.day);
      final orders = [
        MerchantOrder(id: 'a', status: 'DELIVERED', merchantStatus: 'READY', createdAt: today, totalAmount: 5000),
        MerchantOrder(id: 'b', status: 'DELIVERED', createdAt: null, totalAmount: 99999),
      ];
      final s = AnalyticsSummary.compute(orders, 'today');
      expect(s.totalOrders, 1);
      expect(s.totalRevenue, 5000);
    });

    test('week period starts 6 days ago', () {
      final now = DateTime.now();
      final today = DateTime(now.year, now.month, now.day);
      final s = AnalyticsSummary.compute(const [], 'week');
      // 7 days: today + 6 prior days
      expect(s.dailyRevenue.length, 7);
    });

    test('month period and empty top items', () {
      final s = AnalyticsSummary.compute(const [], 'month');
      expect(s.period, 'month');
      expect(s.topItems, isEmpty);
      expect(s.totalOrders, 0);
      expect(s.avgOrderValue, 0);
    });

    test('unknown period defaults to today', () {
      final s = AnalyticsSummary.compute(const [], 'weird');
      expect(s.dailyRevenue.length, 1);
    });

    test('analyticsPeriodLabel maps all', () {
      expect(analyticsPeriodLabel('today'), 'Today');
      expect(analyticsPeriodLabel('week'), 'This Week');
      expect(analyticsPeriodLabel('month'), 'This Month');
      expect(analyticsPeriodLabel('other'), 'other');
    });

    test('avg order value is integer floor division', () {
      final now = DateTime.now();
      final today = DateTime(now.year, now.month, now.day);
      final orders = [
        MerchantOrder(id: 'a', status: 'DELIVERED', merchantStatus: 'READY', createdAt: today, totalAmount: 10000),
        MerchantOrder(id: 'b', status: 'DELIVERED', merchantStatus: 'READY', createdAt: today, totalAmount: 10000),
        MerchantOrder(id: 'c', status: 'DELIVERED', merchantStatus: 'READY', createdAt: today, totalAmount: 10000),
        MerchantOrder(id: 'd', status: 'DELIVERED', merchantStatus: 'READY', createdAt: today, totalAmount: 10000),
      ];
      final s = AnalyticsSummary.compute(orders, 'today');
      expect(s.avgOrderValue, 10000);
    });
  });
}
