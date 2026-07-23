import 'package:flutter/material.dart';

/// Logo horizontal de Obertrack (isotipo + "Obertrack" + tagline). Elige la
/// variante blanca o a color según el brillo del fondo (o se fuerza con
/// [forceWhite]).
class AppLogo extends StatelessWidget {
  const AppLogo({super.key, this.height = 44, this.forceWhite});

  final double height;
  final bool? forceWhite;

  @override
  Widget build(BuildContext context) {
    final white = forceWhite ?? Theme.of(context).brightness == Brightness.dark;
    return Image.asset(
      white ? 'assets/logo/obertrack-white.png' : 'assets/logo/obertrack-color.png',
      height: height,
      fit: BoxFit.contain,
      filterQuality: FilterQuality.high,
    );
  }
}

/// Isotipo (marca cuadrada a color), útil como marca compacta o en el splash.
class AppIsotipo extends StatelessWidget {
  const AppIsotipo({super.key, this.size = 72});
  final double size;

  @override
  Widget build(BuildContext context) {
    return Image.asset(
      'assets/logo/isotipo.png',
      width: size,
      height: size,
      fit: BoxFit.contain,
      filterQuality: FilterQuality.high,
    );
  }
}

/// Firma "powered by Oberstaff" (Obertrack es un producto de Oberstaff).
class PoweredByOberstaff extends StatelessWidget {
  const PoweredByOberstaff({super.key, this.forceWhite, this.logoHeight = 16});

  final bool? forceWhite;
  final double logoHeight;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final white = forceWhite ?? theme.brightness == Brightness.dark;
    final labelColor = (white ? Colors.white : theme.colorScheme.onSurface)
        .withValues(alpha: 0.6);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          'powered by',
          style: TextStyle(
            color: labelColor,
            fontSize: 11,
            letterSpacing: 0.4,
          ),
        ),
        const SizedBox(width: 6),
        // El logo de Oberstaff solo existe en blanco; en fondo claro lo
        // atenuamos con opacidad para que se lea sobre el fondo.
        Opacity(
          opacity: white ? 0.85 : 0.55,
          child: ColorFiltered(
            colorFilter: white
                ? const ColorFilter.mode(Colors.transparent, BlendMode.dst)
                : const ColorFilter.mode(Colors.black87, BlendMode.srcIn),
            child: Image.asset(
              'assets/logo/oberstaff-white.png',
              height: logoHeight,
              fit: BoxFit.contain,
              filterQuality: FilterQuality.high,
            ),
          ),
        ),
      ],
    );
  }
}
