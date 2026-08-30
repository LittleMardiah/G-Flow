import 'package:flutter/material.dart';

import '../models/driver_order.dart';

/// Badge tipe order berwarna: Ride (biru), Food (oranye), Send (hijau) —
/// konsisten dengan ROADMAP 3.10 A.
class OrderTypeBadge extends StatelessWidget {
  const OrderTypeBadge({super.key, required this.type});

  final OrderType type;

  static const Map<OrderType, (Color, IconData)> _kTheme = {
    OrderType.ride: (Colors.blue, Icons.directions_car),
    OrderType.food: (Colors.orange, Icons.restaurant),
    OrderType.send: (Colors.green, Icons.inventory_2),
  };

  @override
  Widget build(BuildContext context) {
    final (color, icon) = _kTheme[type]!;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.14),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 13, color: color),
          const SizedBox(width: 4),
          Text(
            type.label.toUpperCase(),
            style: TextStyle(
              color: color,
              fontSize: 11,
              fontWeight: FontWeight.bold,
              letterSpacing: 0.4,
            ),
          ),
        ],
      ),
    );
  }
}