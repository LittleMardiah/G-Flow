import 'package:flutter_test/flutter_test.dart';

import 'package:customer_app/models/food_order.dart';
import 'package:customer_app/models/item_option.dart';
import 'package:customer_app/models/merchant.dart';
import 'package:customer_app/models/menu_item.dart';
import 'package:customer_app/models/option_choice.dart';
import 'package:customer_app/models/send_order.dart';
import 'package:customer_app/providers/cart_provider.dart';

void main() {
  group('Merchant.fromJson', () {
    test('mengurai field merchant sesuai API_CONTRACT 8.1', () {
      final m = Merchant.fromJson(const {
        'merchant_id': 'uuid-merchant-001',
        'name': 'Warung Mak Esah',
        'latitude': -6.2100,
        'longitude': 106.8450,
        'distance_km': 0.5,
        'rating': 4.7,
        'average_delivery_time': 25,
        'is_online': true,
      });
      expect(m.id, 'uuid-merchant-001');
      expect(m.name, 'Warung Mak Esah');
      expect(m.latitude, -6.21);
      expect(m.longitude, 106.845);
      expect(m.rating, 4.7);
      expect(m.distanceKm, 0.5);
      expect(m.isOpen, isTrue);
    });
  });

  group('MenuItem.fromJson', () {
    test('mengurai item + opsi multi-grup (HALAMAN 4.2)', () {
      final item = MenuItem.fromJson(const {
        'item_id': 'uuid-item-001',
        'name': 'Nasi Kuning Ayam',
        'price': 15000,
        'available': true,
        'option_groups': [
          {
            'option_group_id': 'uuid-opt-group-001',
            'name': 'Level Pedas',
            'required': true,
            'selection_type': 'SINGLE',
            'options': [
              {'option_id': 'uuid-opt-001', 'name': 'Pedas', 'price_adjust': 0},
              {'option_id': 'uuid-opt-002', 'name': 'Sangat Pedas', 'price_adjust': 2000},
            ],
          }
        ],
      });
      expect(item.id, 'uuid-item-001');
      expect(item.price, 15000);
      expect(item.options.length, 1);
      final group = item.options.first;
      expect(group.name, 'Level Pedas');
      expect(group.isRequired, isTrue);
      expect(group.isMultiple, isFalse);
      expect(group.options.first.priceAdjustment, 0);
      expect(group.options.last.priceAdjustment, 2000);
    });
  });

  group('FoodOrder.fromJson', () {
    test('mengurai field order + items', () {
      final o = FoodOrder.fromJson(const {
        'order_id': 'uuid-food-order-456',
        'merchant_id': 'uuid-merchant-001',
        'status': 'PREPARING',
        'items': [
          {
            'order_item_id': 'uuid-order-item-001',
            'item_name': 'Nasi Kuning Ayam',
            'quantity': 2,
            'unit_price': 15000,
            'subtotal': 30000,
            'selected_options': 'Pedas',
          }
        ],
        'subtotal_amount': 30000,
        'delivery_fee': 10000,
        'discount_amount': 0,
        'total_amount': 40000,
        'payment_method': 'WALLET',
      });
      expect(o.id, 'uuid-food-order-456');
      expect(o.status, 'PREPARING');
      expect(o.items.single.quantity, 2);
      expect(o.items.single.subtotal, 30000);
      expect(o.items.single.selectedOptions, ['Pedas']);
      expect(o.totalAmount, 40000);
      expect(o.deliveryFee, 10000);
      expect(o.isTerminal, isFalse);
    });

    test('status terminal bukan anggota flow', () {
      final o = FoodOrder(
        id: 'x',
        status: 'CANCELLED',
        items: const [],
      );
      expect(o.isTerminal, isTrue);
    });
  });

  group('SendOrder.fromJson', () {
    test('mengurai order + stops + allocated_fare', () {
      final o = SendOrder.fromJson(const {
        'order_id': 'uuid-send-order-123',
        'status': 'IN_TRANSIT',
        'package_type': 'STANDARD',
        'total_distance_km': 13.0,
        'total_fare': 52000,
        'payment_method': 'WALLET',
        'stops': [
          {
            'stop_id': 'uuid-stop-001',
            'order': 1,
            'recipient_name': 'Budi',
            'address': 'Jalan Sudirman No 5',
            'distance_km': 5.0,
            'allocated_fare': 20000,
            'status': 'COMPLETED',
          },
          {
            'stop_id': 'uuid-stop-002',
            'order': 2,
            'recipient_name': 'Ani',
            'address': 'Jalan Thamrin No 10',
            'distance_km': 8.0,
            'allocated_fare': 32000,
            'status': 'PENDING',
          },
        ],
      });
      expect(o.id, 'uuid-send-order-123');
      expect(o.totalAmount, 52000);
      expect(o.stops.length, 2);
      expect(o.stops.first.isDelivered, isTrue);
      expect(o.stops.last.isDelivered, isFalse);
      expect(o.stopsCompleted, 1);
    });
  });

  group('CartNotifier', () {
    test('tambah item, ubah kuantitas, hapus, subtotal', () {
      final cart = CartNotifier();
      final item = MenuItem(
        id: 'item-1',
        merchantId: 'merchant-1',
        name: 'Nasi Kuning',
        price: 15000,
      );
      const group = ItemOptionGroup(
        id: 'group-1',
        name: 'Topping',
        selectionType: 'SINGLE',
        options: [OptionChoice(id: 'opt-extra', name: 'Extra Cheese', priceAdjustment: 5000)],
      );
      const extra = ItemSelection(
        group: group,
        choice: OptionChoice(id: 'opt-extra', name: 'Extra Cheese', priceAdjustment: 5000),
      );

      cart.addItem(item, const [extra], quantity: 2);
      expect(cart.state.lines.length, 1);
      expect(cart.state.itemCount, 2);
      expect(cart.state.subtotal, (15000 + 5000) * 2);
      expect(cart.state.merchantId, 'merchant-1');

      // Kuantitas naik.
      cart.updateQuantity(cart.state.lines.first, 3);
      expect(cart.state.itemCount, 3);

      // Hapus.
      cart.removeItem(cart.state.lines.first);
      expect(cart.state.lines, isEmpty);
      expect(cart.state.subtotal, 0);
    });

    test('baris sama (item + opsi sama) digabung bukan duplikat', () {
      final cart = CartNotifier();
      final a = MenuItem(id: 'item-1', merchantId: 'm', name: 'A', price: 1000);
      final b = MenuItem(id: 'item-2', merchantId: 'm', name: 'B', price: 2000);
      cart.addItem(a, const []);
      cart.addItem(a, const []);
      cart.addItem(b, const []);
      expect(cart.state.lines.length, 2);
      expect(cart.state.itemCount, 3);
    });
  });
}